// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package agent

import (
	"context"
	stdcontext "context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/juju/charm/v9"
	"github.com/juju/clock"
	"github.com/juju/cmd/v3"
	"github.com/juju/cmd/v3/cmdtesting"
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/mgo/v2"
	"github.com/juju/names/v4"
	gitjujutesting "github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/utils/v2"
	"github.com/juju/version/v2"
	gc "gopkg.in/check.v1"
	"gopkg.in/macaroon.v2"

	"github.com/juju/juju/agent"
	"github.com/juju/juju/agent/agentbootstrap"
	agenttools "github.com/juju/juju/agent/tools"
	"github.com/juju/juju/apiserver/facades/client/charms/interfaces"
	"github.com/juju/juju/apiserver/facades/client/charms/mocks"
	"github.com/juju/juju/apiserver/facades/client/charms/services"
	"github.com/juju/juju/cloud"
	"github.com/juju/juju/cloudconfig/instancecfg"
	"github.com/juju/juju/cmd/jujud/agent/agenttest"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/controller"
	corecharm "github.com/juju/juju/core/charm"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/network"
	coreos "github.com/juju/juju/core/os"
	"github.com/juju/juju/core/series"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/environs/config"
	envcontext "github.com/juju/juju/environs/context"
	"github.com/juju/juju/environs/filestorage"
	"github.com/juju/juju/environs/imagemetadata"
	"github.com/juju/juju/environs/instances"
	"github.com/juju/juju/environs/simplestreams"
	sstesting "github.com/juju/juju/environs/simplestreams/testing"
	"github.com/juju/juju/environs/storage"
	envtesting "github.com/juju/juju/environs/testing"
	envtools "github.com/juju/juju/environs/tools"
	"github.com/juju/juju/juju/keys"
	jujutesting "github.com/juju/juju/juju/testing"
	"github.com/juju/juju/mongo"
	"github.com/juju/juju/mongo/mongotest"
	"github.com/juju/juju/provider/dummy"
	"github.com/juju/juju/state"
	"github.com/juju/juju/state/cloudimagemetadata"
	"github.com/juju/juju/testcharms"
	"github.com/juju/juju/testing"
	"github.com/juju/juju/tools"
	jujuversion "github.com/juju/juju/version"
)

// We don't want to use JujuConnSuite because it gives us
// an already-bootstrapped environment.
type BootstrapSuite struct {
	testing.BaseSuite
	gitjujutesting.MgoSuite

	bootstrapParamsFile string
	bootstrapParams     instancecfg.StateInitializationParams

	dataDir         string
	logDir          string
	mongoOplogSize  string
	fakeEnsureMongo *agenttest.FakeEnsureMongo
	bootstrapName   string
	hostedModelUUID string

	toolsStorage storage.Storage
}

var _ = gc.Suite(&BootstrapSuite{})

func (s *BootstrapSuite) SetUpSuite(c *gc.C) {
	storageDir := c.MkDir()
	restorer := gitjujutesting.PatchValue(&envtools.DefaultBaseURL, storageDir)
	stor, err := filestorage.NewFileStorageWriter(storageDir)
	c.Assert(err, jc.ErrorIsNil)
	s.toolsStorage = stor

	s.BaseSuite.SetUpSuite(c)
	s.AddCleanup(func(*gc.C) {
		restorer()
	})
	s.MgoSuite.SetUpSuite(c)
	s.PatchValue(&jujuversion.Current, testing.FakeVersionNumber)
}

func (s *BootstrapSuite) TearDownSuite(c *gc.C) {
	s.MgoSuite.TearDownSuite(c)
	s.BaseSuite.TearDownSuite(c)
	dummy.Reset(c)
}

func (s *BootstrapSuite) SetUpTest(c *gc.C) {
	s.BaseSuite.SetUpTest(c)
	s.PatchValue(&sshGenerateKey, func(name string) (string, string, error) {
		return "private-key", "public-key", nil
	})
	s.PatchValue(&series.UbuntuDistroInfo, "/path/notexists")

	s.MgoSuite.SetUpTest(c)
	s.dataDir = c.MkDir()
	s.logDir = c.MkDir()
	s.bootstrapParamsFile = filepath.Join(s.dataDir, "bootstrap-params")
	s.mongoOplogSize = "1234"
	s.fakeEnsureMongo = agenttest.InstallFakeEnsureMongo(s, s.dataDir)
	s.PatchValue(&initiateMongoServer, s.fakeEnsureMongo.InitiateMongo)
	s.makeTestModel(c)

	// Create fake tools.tar.gz and downloaded-tools.txt.
	current := testing.CurrentVersion(c)
	toolsDir := filepath.FromSlash(agenttools.SharedToolsDir(s.dataDir, current))
	err := os.MkdirAll(toolsDir, 0755)
	c.Assert(err, jc.ErrorIsNil)
	err = ioutil.WriteFile(filepath.Join(toolsDir, "tools.tar.gz"), nil, 0644)
	c.Assert(err, jc.ErrorIsNil)
	s.writeDownloadedTools(c, &tools.Tools{Version: current})

	// Create fake dashboard.tar.bz2 and downloaded-dashboard.txt.
	dashboardDir := filepath.FromSlash(agenttools.SharedDashboardDir(s.dataDir))
	err = os.MkdirAll(dashboardDir, 0755)
	c.Assert(err, jc.ErrorIsNil)
	err = ioutil.WriteFile(filepath.Join(dashboardDir, "dashboard.tar.bz2"), nil, 0644)
	c.Assert(err, jc.ErrorIsNil)
	s.writeDownloadedDashboard(c, &tools.DashboardArchive{
		Version: version.MustParse("2.0.42"),
	})

	// Create fake local controller charm.
	controllerCharmPath := filepath.Join(s.dataDir, "charms")
	err = os.MkdirAll(controllerCharmPath, 0755)
	c.Assert(err, jc.ErrorIsNil)
	pathToArchive := testcharms.Repo.CharmArchivePath(controllerCharmPath, "juju-controller")
	err = os.Rename(pathToArchive, filepath.Join(controllerCharmPath, "controller.charm"))
	c.Assert(err, jc.ErrorIsNil)
}

func (s *BootstrapSuite) TearDownTest(c *gc.C) {
	s.MgoSuite.TearDownTest(c)
	s.BaseSuite.TearDownTest(c)
}

func (s *BootstrapSuite) writeDownloadedTools(c *gc.C, tools *tools.Tools) {
	toolsDir := filepath.FromSlash(agenttools.SharedToolsDir(s.dataDir, tools.Version))
	err := os.MkdirAll(toolsDir, 0755)
	c.Assert(err, jc.ErrorIsNil)
	data, err := json.Marshal(tools)
	c.Assert(err, jc.ErrorIsNil)
	err = ioutil.WriteFile(filepath.Join(toolsDir, "downloaded-tools.txt"), data, 0644)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *BootstrapSuite) writeDownloadedDashboard(c *gc.C, dashboard *tools.DashboardArchive) {
	dashboardDir := filepath.FromSlash(agenttools.SharedDashboardDir(s.dataDir))
	err := os.MkdirAll(dashboardDir, 0755)
	c.Assert(err, jc.ErrorIsNil)
	data, err := json.Marshal(dashboard)
	c.Assert(err, jc.ErrorIsNil)
	err = ioutil.WriteFile(filepath.Join(dashboardDir, "downloaded-dashboard.txt"), data, 0644)
	c.Assert(err, jc.ErrorIsNil)
}

func (s *BootstrapSuite) TestDashboardArchiveInfoNotFound(c *gc.C) {
	dir := filepath.FromSlash(agenttools.SharedDashboardDir(s.dataDir))
	info := filepath.Join(dir, "downloaded-dashboard.txt")
	err := os.Remove(info)
	c.Assert(err, jc.ErrorIsNil)
	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)

	var tw loggo.TestWriter
	err = loggo.RegisterWriter("bootstrap-test", &tw)
	c.Assert(err, jc.ErrorIsNil)
	defer loggo.RemoveWriter("bootstrap-test")

	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(tw.Log(), jc.LogMatches, jc.SimpleMessages{{
		loggo.WARNING,
		`cannot set up Juju Dashboard: cannot fetch Dashboard info: Dashboard metadata not found`,
	}})
}

func (s *BootstrapSuite) TestDashboardArchiveInfoError(c *gc.C) {
	if runtime.GOOS == "windows" {
		// TODO frankban: skipping for now due to chmod problems with mode 0000
		// on Windows. We will re-enable this test after further investigation:
		// "jujud bootstrap" is never run on Windows anyway.
		c.Skip("needs chmod investigation")
	}
	dir := filepath.FromSlash(agenttools.SharedDashboardDir(s.dataDir))
	info := filepath.Join(dir, "downloaded-dashboard.txt")
	err := os.Chmod(info, 0000)
	c.Assert(err, jc.ErrorIsNil)
	defer os.Chmod(info, 0600)
	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)

	var tw loggo.TestWriter
	err = loggo.RegisterWriter("bootstrap-test", &tw)
	c.Assert(err, jc.ErrorIsNil)
	defer loggo.RemoveWriter("bootstrap-test")

	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(tw.Log(), jc.LogMatches, jc.SimpleMessages{{
		loggo.WARNING,
		`cannot set up Juju Dashboard: cannot fetch Dashboard info: cannot read Dashboard metadata in directory .*`,
	}})
}

func (s *BootstrapSuite) TestDashboardArchiveError(c *gc.C) {
	dir := filepath.FromSlash(agenttools.SharedDashboardDir(s.dataDir))
	archive := filepath.Join(dir, "dashboard.tar.bz2")
	err := os.Remove(archive)
	c.Assert(err, jc.ErrorIsNil)
	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)

	var tw loggo.TestWriter
	err = loggo.RegisterWriter("bootstrap-test", &tw)
	c.Assert(err, jc.ErrorIsNil)
	defer loggo.RemoveWriter("bootstrap-test")

	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(tw.Log(), jc.LogMatches, jc.SimpleMessages{{
		loggo.WARNING,
		`cannot set up Juju Dashboard: cannot read Dashboard archive: .*`,
	}})
}

func (s *BootstrapSuite) getSystemState(c *gc.C) (*state.State, func()) {
	pool, err := state.OpenStatePool(state.OpenParams{
		Clock:              clock.WallClock,
		ControllerTag:      testing.ControllerTag,
		ControllerModelTag: testing.ModelTag,
		MongoSession:       s.Session,
	})
	c.Assert(err, jc.ErrorIsNil)
	return pool.SystemState(), func() { pool.Close() }
}

func (s *BootstrapSuite) TestDashboardArchiveSuccess(c *gc.C) {
	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)

	var tw loggo.TestWriter
	err = loggo.RegisterWriter("bootstrap-test", &tw)
	c.Assert(err, jc.ErrorIsNil)
	defer loggo.RemoveWriter("bootstrap-test")

	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(tw.Log(), jc.LogMatches, jc.SimpleMessages{{
		loggo.DEBUG,
		`Juju Dashboard successfully set up`,
	}})

	// Retrieve the state so that it is possible to access the Dashboard storage.
	st, closer := s.getSystemState(c)
	defer closer()

	// The Dashboard archive has been uploaded to the Dashboard storage.
	storage, err := st.DashboardStorage()
	c.Assert(err, jc.ErrorIsNil)
	defer storage.Close()
	allMeta, err := storage.AllMetadata()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(allMeta, gc.HasLen, 1)
	c.Assert(allMeta[0].Version, gc.Equals, "2.0.42")

	// The current Dashboard version has been set.
	vers, err := st.DashboardVersion()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(vers.String(), gc.Equals, "2.0.42")
}

func (s *BootstrapSuite) TestLocalControllerCharm(c *gc.C) {
	if runtime.GOOS != "linux" {
		c.Skip("controller charm only supported on Ubuntu")
	}
	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)

	var tw loggo.TestWriter
	err = loggo.RegisterWriter("bootstrap-test", &tw)
	c.Assert(err, jc.ErrorIsNil)
	defer loggo.RemoveWriter("bootstrap-test")

	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(tw.Log(), jc.LogMatches, jc.SimpleMessages{{
		loggo.DEBUG,
		`Successfully deployed local Juju controller charm`,
	}})
	s.assertControllerApplication(c)
}

func (s *BootstrapSuite) TestStoreControllerCharm(c *gc.C) {
	if runtime.GOOS != "linux" {
		c.Skip("controller charm only supported on Ubuntu")
	}
	// Remove the local controller charm so we use the store one.
	controllerCharmPath := filepath.Join(s.dataDir, "charms", "controller.charm")
	err := os.Remove(controllerCharmPath)
	c.Assert(err, jc.ErrorIsNil)

	ctrl := gomock.NewController(c)
	defer ctrl.Finish()
	repo := mocks.NewMockRepository(ctrl)
	s.PatchValue(&newCharmRepo, func(cfg services.CharmRepoFactoryConfig) (corecharm.Repository, error) {
		return repo, nil
	})
	downloader := mocks.NewMockDownloader(ctrl)
	s.PatchValue(&newCharmDownloader, func(cfg services.CharmDownloaderConfig) (interfaces.Downloader, error) {
		return downloader, nil
	})

	curl := charm.MustParseURL(controllerCharmURL)
	channel := corecharm.MakeRiskOnlyChannel("beta")
	origin := corecharm.Origin{
		Source:  corecharm.CharmHub,
		Type:    "charm",
		Channel: &channel,
		Platform: corecharm.Platform{
			Architecture: "amd64",
			OS:           "ubuntu",
			Series:       "NA",
		},
	}

	storeCurl := *curl
	storeCurl.Revision = 666
	storeCurl.Series = "focal"
	storeCurl.Architecture = "amd64"
	storeOrigin := origin
	storeOrigin.Platform.Series = "focal"
	repo.EXPECT().ResolveWithPreferredChannel(curl, origin, nil).Return(&storeCurl, storeOrigin, nil, nil)

	origin.Platform.Series = "focal"
	downloader.EXPECT().DownloadAndStore(&storeCurl, storeOrigin, nil, false).
		DoAndReturn(func(charmURL *charm.URL, requestedOrigin corecharm.Origin, macaroons macaroon.Slice, force bool) (*charm.CharmArchive, error) {
			controllerCharm := testcharms.Repo.CharmArchive(c.MkDir(), "juju-controller")
			st, closer := s.getSystemState(c)
			defer closer()
			_, err = st.AddCharm(state.CharmInfo{
				Charm:       controllerCharm,
				ID:          charmURL,
				StoragePath: "foo", // required to flag the charm as uploaded
				SHA256:      "bar", // required to flag the charm as uploaded
			})
			c.Assert(err, jc.ErrorIsNil)
			return controllerCharm, nil
		})

	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)

	var tw loggo.TestWriter
	err = loggo.RegisterWriter("bootstrap-test", &tw)
	c.Assert(err, jc.ErrorIsNil)
	defer loggo.RemoveWriter("bootstrap-test")

	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(tw.Log(), jc.LogMatches, jc.SimpleMessages{{
		loggo.DEBUG,
		`Successfully deployed store Juju controller charm`,
	}})
	s.assertControllerApplication(c)
}

func (s *BootstrapSuite) assertControllerApplication(c *gc.C) {
	st, closer := s.getSystemState(c)
	defer closer()

	app, err := st.Application("controller")
	c.Assert(err, jc.ErrorIsNil)
	appCh, _, err := app.Charm()
	c.Assert(err, jc.ErrorIsNil)
	stateCh, err := st.Charm(appCh.URL())
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(stateCh.Meta().Name, gc.Equals, "juju-controller")
	units, err := app.AllUnits()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(units, gc.HasLen, 1)
	m, err := units[0].AssignedMachineId()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(m, gc.Equals, "0")
}

var testPassword = "my-admin-secret"

func (s *BootstrapSuite) initBootstrapCommand(c *gc.C, jobs []model.MachineJob, args ...string) (machineConf agent.ConfigSetterWriter, cmd *BootstrapCommand, err error) {
	if len(jobs) == 0 {
		// Add default jobs.
		jobs = []model.MachineJob{
			model.JobManageModel,
			model.JobHostUnits,
		}
	}
	// NOTE: the old test used an equivalent of the NewAgentConfig, but it
	// really should be using NewStateMachineConfig.
	agentParams := agent.AgentConfigParams{
		Paths: agent.Paths{
			LogDir:  s.logDir,
			DataDir: s.dataDir,
		},
		Jobs:              jobs,
		Tag:               names.NewMachineTag("0"),
		UpgradedToVersion: jujuversion.Current,
		Password:          testPassword,
		Nonce:             agent.BootstrapNonce,
		Controller:        testing.ControllerTag,
		Model:             testing.ModelTag,
		APIAddresses:      []string{"127.0.0.2:1234"},
		CACert:            testing.CACert,
		Values: map[string]string{
			agent.Namespace:      "foobar",
			agent.MongoOplogSize: s.mongoOplogSize,
		},
	}
	servingInfo := controller.StateServingInfo{
		Cert:         "some cert",
		PrivateKey:   "some key",
		CAPrivateKey: "another key",
		APIPort:      3737,
		StatePort:    gitjujutesting.MgoServer.Port(),
	}

	machineConf, err = agent.NewStateMachineConfig(agentParams, servingInfo)
	c.Assert(err, jc.ErrorIsNil)
	err = machineConf.Write()
	c.Assert(err, jc.ErrorIsNil)

	if len(args) == 0 {
		args = []string{s.bootstrapParamsFile}
	}
	cmd = NewBootstrapCommand()
	err = cmdtesting.InitCommand(cmd, append([]string{"--data-dir", s.dataDir}, args...))
	return machineConf, cmd, err
}

func (s *BootstrapSuite) TestInitializeEnvironment(c *gc.C) {
	machConf, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)
	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)

	c.Assert(s.fakeEnsureMongo.DataDir, gc.Equals, s.dataDir)
	c.Assert(s.fakeEnsureMongo.InitiateCount, gc.Equals, 1)
	c.Assert(s.fakeEnsureMongo.EnsureCount, gc.Equals, 1)
	c.Assert(s.fakeEnsureMongo.OplogSize, gc.Equals, 1234)

	expectInfo, exists := machConf.StateServingInfo()
	c.Assert(exists, jc.IsTrue)
	c.Assert(expectInfo.SharedSecret, gc.Equals, "")
	c.Assert(expectInfo.SystemIdentity, gc.Equals, "")

	servingInfo := s.fakeEnsureMongo.Info
	c.Assert(len(servingInfo.SharedSecret), gc.Not(gc.Equals), 0)
	c.Assert(len(servingInfo.SystemIdentity), gc.Not(gc.Equals), 0)
	servingInfo.SharedSecret = ""
	servingInfo.SystemIdentity = ""
	c.Assert(servingInfo, jc.DeepEquals, expectInfo)
	expectDialAddrs := []string{fmt.Sprintf("localhost:%d", expectInfo.StatePort)}
	gotDialAddrs := s.fakeEnsureMongo.InitiateParams.DialInfo.Addrs
	c.Assert(gotDialAddrs, gc.DeepEquals, expectDialAddrs)

	c.Assert(
		s.fakeEnsureMongo.InitiateParams.MemberHostPort,
		gc.Matches,
		fmt.Sprintf("only-0.dns:%d$", expectInfo.StatePort),
	)
	c.Assert(s.fakeEnsureMongo.InitiateParams.User, gc.Equals, "")
	c.Assert(s.fakeEnsureMongo.InitiateParams.Password, gc.Equals, "")

	st, closer := s.getSystemState(c)
	defer closer()
	machines, err := st.AllMachines()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(machines, gc.HasLen, 1)

	instid, err := machines[0].InstanceId()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(instid, gc.Equals, s.bootstrapParams.BootstrapMachineInstanceId)

	stateHw, err := machines[0].HardwareCharacteristics()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(stateHw, gc.NotNil)
	c.Assert(stateHw, gc.DeepEquals, s.bootstrapParams.BootstrapMachineHardwareCharacteristics)

	cons, err := st.ModelConstraints()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(&cons, jc.Satisfies, constraints.IsEmpty)

	m, err := st.Model()
	c.Assert(err, jc.ErrorIsNil)

	cfg, err := m.ModelConfig()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(cfg.AuthorizedKeys(), gc.Equals, s.bootstrapParams.ControllerModelConfig.AuthorizedKeys()+"\npublic-key")
}

func (s *BootstrapSuite) TestInitializeEnvironmentInvalidOplogSize(c *gc.C) {
	s.mongoOplogSize = "NaN"
	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)
	err = cmd.Run(nil)
	c.Assert(err, gc.ErrorMatches, `failed to start mongo: invalid oplog size: "NaN"`)
}

func (s *BootstrapSuite) TestInitializeEnvironmentToolsNotFound(c *gc.C) {
	// bootstrap with 2.99.1 but there will be no tools so version will be reset.
	cfg, err := s.bootstrapParams.ControllerModelConfig.Apply(map[string]interface{}{
		"agent-version": "2.99.1",
	})
	c.Assert(err, jc.ErrorIsNil)
	s.bootstrapParams.ControllerModelConfig = cfg
	s.writeBootstrapParamsFile(c)

	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)
	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)

	st, closer := s.getSystemState(c)
	defer closer()

	m, err := st.Model()
	c.Assert(err, jc.ErrorIsNil)

	cfg, err = m.ModelConfig()
	c.Assert(err, jc.ErrorIsNil)
	vers, ok := cfg.AgentVersion()
	c.Assert(ok, jc.IsTrue)
	c.Assert(vers.String(), gc.Equals, "2.99.0")
}

func (s *BootstrapSuite) TestSetConstraints(c *gc.C) {
	s.bootstrapParams.BootstrapMachineConstraints = constraints.Value{Mem: uint64p(4096), CpuCores: uint64p(4)}
	s.bootstrapParams.ModelConstraints = constraints.Value{Mem: uint64p(2048), CpuCores: uint64p(2)}
	s.writeBootstrapParamsFile(c)

	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)
	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)

	st, closer := s.getSystemState(c)
	defer closer()

	cons, err := st.ModelConstraints()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(cons, gc.DeepEquals, s.bootstrapParams.ModelConstraints)

	machines, err := st.AllMachines()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(machines, gc.HasLen, 1)
	cons, err = machines[0].Constraints()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(cons, gc.DeepEquals, s.bootstrapParams.BootstrapMachineConstraints)
}

func uint64p(v uint64) *uint64 {
	return &v
}

func (s *BootstrapSuite) TestDefaultMachineJobs(c *gc.C) {
	expectedJobs := []state.MachineJob{
		state.JobManageModel,
		state.JobHostUnits,
	}
	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)
	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)

	st, closer := s.getSystemState(c)
	defer closer()
	m, err := st.Machine("0")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(m.Jobs(), gc.DeepEquals, expectedJobs)
}

func (s *BootstrapSuite) TestInitialPassword(c *gc.C) {
	machineConf, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)

	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)

	// Check we can log in to mongo as admin.
	info := mongo.MongoInfo{
		Info: mongo.Info{
			Addrs:      []string{gitjujutesting.MgoServer.Addr()},
			CACert:     testing.CACert,
			DisableTLS: !gitjujutesting.MgoServer.SSLEnabled(),
		},
		Tag:      nil, // admin user
		Password: testPassword,
	}
	session, err := mongo.DialWithInfo(info, mongotest.DialOpts())
	c.Assert(err, jc.ErrorIsNil)
	defer session.Close()

	// We're running Mongo with --noauth; let's explicitly verify
	// that we can login as that user. Even with --noauth, an
	// explicit Login will still be verified.
	adminDB := session.DB("admin")
	err = adminDB.Login("admin", "invalid-password")
	c.Assert(err, gc.ErrorMatches, "(auth|(.*Authentication)) fail(s|ed)\\.?")
	err = adminDB.Login("admin", info.Password)
	c.Assert(err, jc.ErrorIsNil)

	// Check that the admin user has been given an appropriate password
	st, closer := s.getSystemState(c)
	defer closer()
	u, err := st.User(names.NewLocalUserTag("admin"))
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(u.PasswordValid(testPassword), jc.IsTrue)

	// Check that the machine configuration has been given a new
	// password and that we can connect to mongo as that machine
	// and that the in-mongo password also verifies correctly.
	machineConf1, err := agent.ReadConfig(agent.ConfigPath(machineConf.DataDir(), names.NewMachineTag("0")))
	c.Assert(err, jc.ErrorIsNil)

	machineMongoInfo, ok := machineConf1.MongoInfo()
	c.Assert(ok, jc.IsTrue)
	session, err = mongo.DialWithInfo(*machineMongoInfo, mongotest.DialOpts())
	c.Assert(err, jc.ErrorIsNil)
	defer session.Close()

	st, closer = s.getSystemState(c)
	defer closer()

	node, err := st.ControllerNode("0")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(node.HasVote(), jc.IsTrue)
}

var bootstrapArgTests = []struct {
	input                       []string
	err                         string
	expectedBootstrapParamsFile string
}{
	{
		err:   "bootstrap-params file must be specified",
		input: []string{"--data-dir", "/tmp/juju/data/dir"},
	}, {
		input:                       []string{"/some/where"},
		expectedBootstrapParamsFile: "/some/where",
	},
}

func (s *BootstrapSuite) TestBootstrapArgs(c *gc.C) {
	for i, t := range bootstrapArgTests {
		c.Logf("test %d", i)
		var args []string
		args = append(args, t.input...)
		_, cmd, err := s.initBootstrapCommand(c, nil, args...)
		if t.err == "" {
			c.Assert(cmd, gc.NotNil)
			c.Assert(err, jc.ErrorIsNil)
			c.Assert(cmd.BootstrapParamsFile, gc.Equals, t.expectedBootstrapParamsFile)
		} else {
			c.Assert(err, gc.ErrorMatches, t.err)
		}
	}
}

func (s *BootstrapSuite) TestInitializeStateArgs(c *gc.C) {
	var called int
	initializeState := func(
		_ environs.BootstrapEnviron,
		_ names.UserTag,
		_ agent.ConfigSetter,
		args agentbootstrap.InitializeStateParams,
		dialOpts mongo.DialOpts,
		_ state.NewPolicyFunc,
	) (_ *state.Controller, resultErr error) {
		called++
		c.Assert(dialOpts.Direct, jc.IsTrue)
		c.Assert(dialOpts.Timeout, gc.Equals, 30*time.Second)
		c.Assert(dialOpts.SocketTimeout, gc.Equals, 123*time.Second)
		c.Assert(args.HostedModelConfig, jc.DeepEquals, map[string]interface{}{
			"name": "hosted-model",
			"uuid": s.hostedModelUUID,
		})
		return nil, errors.New("failed to initialize state")
	}
	s.PatchValue(&agentInitializeState, initializeState)
	_, cmd, err := s.initBootstrapCommand(c, nil, "--timeout", "123s", s.bootstrapParamsFile)
	c.Assert(err, jc.ErrorIsNil)
	err = cmd.Run(nil)
	c.Assert(err, gc.ErrorMatches, "failed to initialize state")
	c.Assert(called, gc.Equals, 1)
}

func (s *BootstrapSuite) TestInitializeStateMinSocketTimeout(c *gc.C) {
	var called int
	initializeState := func(
		_ environs.BootstrapEnviron,
		_ names.UserTag,
		_ agent.ConfigSetter,
		_ agentbootstrap.InitializeStateParams,
		dialOpts mongo.DialOpts,
		_ state.NewPolicyFunc,
	) (_ *state.Controller, resultErr error) {
		called++
		c.Assert(dialOpts.Direct, jc.IsTrue)
		c.Assert(dialOpts.SocketTimeout, gc.Equals, 1*time.Minute)
		return nil, errors.New("failed to initialize state")
	}

	s.PatchValue(&agentInitializeState, initializeState)
	_, cmd, err := s.initBootstrapCommand(c, nil, "--timeout", "13s", s.bootstrapParamsFile)
	c.Assert(err, jc.ErrorIsNil)
	err = cmd.Run(nil)
	c.Assert(err, gc.ErrorMatches, "failed to initialize state")
	c.Assert(called, gc.Equals, 1)
}

func (s *BootstrapSuite) TestBootstrapWithInvalidCredentialLogs(c *gc.C) {
	called := false
	newEnviron := func(_ stdcontext.Context, ps environs.OpenParams) (environs.Environ, error) {
		called = true
		env, _ := environs.New(context.TODO(), ps)
		return &mockDummyEnviron{env}, nil
	}
	s.PatchValue(&environsNewIAAS, newEnviron)
	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)
	err = cmd.Run(nil)

	c.Assert(err, jc.ErrorIsNil)
	c.Assert(called, jc.IsTrue)
	// Note that the credential is not needed for dummy provider
	// which is what the test here uses. This test only checks that
	// the message related to the credential is logged.
	c.Assert(c.GetTestLog(), jc.Contains,
		`ERROR juju.cmd.jujud Cloud credential "" is not accepted by cloud provider: considered invalid for the sake of testing`)
}

func (s *BootstrapSuite) TestSystemIdentityWritten(c *gc.C) {
	_, err := os.Stat(filepath.Join(s.dataDir, agent.SystemIdentity))
	c.Assert(err, jc.Satisfies, os.IsNotExist)

	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)
	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)

	data, err := ioutil.ReadFile(filepath.Join(s.dataDir, agent.SystemIdentity))
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(string(data), gc.Equals, "private-key")
}

func (s *BootstrapSuite) TestDownloadedToolsMetadata(c *gc.C) {
	// Tools downloaded by cloud-init script.
	s.testToolsMetadata(c)
}

func (s *BootstrapSuite) TestUploadedToolsMetadata(c *gc.C) {
	// Tools uploaded over ssh.
	s.writeDownloadedTools(c, &tools.Tools{
		Version: testing.CurrentVersion(c),
		URL:     "file:///does/not/matter",
	})
	s.testToolsMetadata(c)
}

func (s *BootstrapSuite) testToolsMetadata(c *gc.C) {
	envtesting.RemoveFakeToolsMetadata(c, s.toolsStorage)

	_, cmd, err := s.initBootstrapCommand(c, nil)

	c.Assert(err, jc.ErrorIsNil)
	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)

	// We don't write metadata at bootstrap anymore.
	ss := simplestreams.NewSimpleStreams(sstesting.TestDataSourceFactory())
	simplestreamsMetadata, err := envtools.ReadMetadata(ss, s.toolsStorage, "released")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(simplestreamsMetadata, gc.HasLen, 0)

	// The tools should have been added to tools storage.
	st, closer := s.getSystemState(c)
	defer closer()

	storage, err := st.ToolsStorage()
	c.Assert(err, jc.ErrorIsNil)
	defer storage.Close()
	metadata, err := storage.AllMetadata()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(metadata, gc.HasLen, 1)
	m := metadata[0]
	v := version.MustParseBinary(m.Version)
	c.Assert(v.Release, gc.Equals, coreos.HostOSTypeName())
}

func createImageMetadata() []*imagemetadata.ImageMetadata {
	return []*imagemetadata.ImageMetadata{{
		Id:         "imageId",
		Storage:    "rootStore",
		VirtType:   "virtType",
		Arch:       "amd64",
		Version:    "14.04",
		Endpoint:   "endpoint",
		RegionName: "region",
	}}
}

func (s *BootstrapSuite) assertWrittenToState(c *gc.C, session *mgo.Session, metadata cloudimagemetadata.Metadata) {
	st, closer := s.getSystemState(c)
	defer closer()

	// find all image metadata in state
	all, err := st.CloudImageMetadataStorage.FindMetadata(cloudimagemetadata.MetadataFilter{})
	c.Assert(err, jc.ErrorIsNil)
	// if there was no stream, it should have defaulted to "released"
	if metadata.Stream == "" {
		metadata.Stream = "released"
	}
	if metadata.DateCreated == 0 && len(all[metadata.Source]) > 0 {
		metadata.DateCreated = all[metadata.Source][0].DateCreated
	}
	c.Assert(all, gc.DeepEquals, map[string][]cloudimagemetadata.Metadata{
		metadata.Source: {metadata},
	})
}

func (s *BootstrapSuite) TestStructuredImageMetadataStored(c *gc.C) {
	s.bootstrapParams.CustomImageMetadata = createImageMetadata()
	s.writeBootstrapParamsFile(c)
	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)
	err = cmd.Run(nil)
	c.Assert(err, jc.ErrorIsNil)

	// This metadata should have also been written to state...
	expect := cloudimagemetadata.Metadata{
		MetadataAttributes: cloudimagemetadata.MetadataAttributes{
			Region:          "region",
			Arch:            "amd64",
			Version:         "14.04",
			Series:          "trusty",
			RootStorageType: "rootStore",
			VirtType:        "virtType",
			Source:          "custom",
		},
		Priority: simplestreams.CUSTOM_CLOUD_DATA,
		ImageId:  "imageId",
	}
	s.assertWrittenToState(c, s.Session, expect)
}

func (s *BootstrapSuite) TestStructuredImageMetadataInvalidSeries(c *gc.C) {
	s.bootstrapParams.CustomImageMetadata = createImageMetadata()
	s.bootstrapParams.CustomImageMetadata[0].Version = "woat"
	s.writeBootstrapParamsFile(c)

	_, cmd, err := s.initBootstrapCommand(c, nil)
	c.Assert(err, jc.ErrorIsNil)
	err = cmd.Run(nil)
	c.Assert(err, gc.ErrorMatches, `cannot determine series for version woat: unknown series for version: \"woat\"`)
}

func (s *BootstrapSuite) makeTestModel(c *gc.C) {
	attrs := dummy.SampleConfig().Merge(
		testing.Attrs{
			"agent-version": jujuversion.Current.String(),
		},
	).Delete("admin-secret", "ca-private-key")
	cfg, err := config.New(config.NoDefaults, attrs)
	c.Assert(err, jc.ErrorIsNil)
	provider, err := environs.Provider(cfg.Type())
	c.Assert(err, jc.ErrorIsNil)
	controllerCfg := testing.FakeControllerConfig()
	cfg, err = provider.PrepareConfig(environs.PrepareConfigParams{
		Config: cfg,
	})
	c.Assert(err, jc.ErrorIsNil)
	env, err := environs.Open(context.TODO(), provider, environs.OpenParams{
		Cloud:  dummy.SampleCloudSpec(),
		Config: cfg,
	})
	c.Assert(err, jc.ErrorIsNil)
	err = env.PrepareForBootstrap(nullContext(), "controller-1")
	c.Assert(err, jc.ErrorIsNil)

	callCtx := envcontext.NewEmptyCloudCallContext()
	s.AddCleanup(func(c *gc.C) {
		err := env.DestroyController(callCtx, controllerCfg.ControllerUUID())
		c.Assert(err, jc.ErrorIsNil)
	})

	s.PatchValue(&keys.JujuPublicKey, sstesting.SignedMetadataPublicKey)
	envtesting.UploadFakeTools(c, s.toolsStorage, cfg.AgentStream(), cfg.AgentStream())
	inst, _, _, err := jujutesting.StartInstance(env, callCtx, testing.FakeControllerConfig().ControllerUUID(), "0")
	c.Assert(err, jc.ErrorIsNil)

	addresses, err := inst.Addresses(callCtx)
	c.Assert(err, jc.ErrorIsNil)
	addr, _ := addresses.OneMatchingScope(network.ScopeMatchPublic)
	s.bootstrapName = addr.Value
	s.hostedModelUUID = utils.MustNewUUID().String()

	var args instancecfg.StateInitializationParams
	args.ControllerConfig = controllerCfg
	args.BootstrapMachineInstanceId = inst.Id()
	args.ControllerModelConfig = env.Config()
	hw := instance.MustParseHardware("arch=amd64 mem=8G")
	args.BootstrapMachineHardwareCharacteristics = &hw
	args.HostedModelConfig = map[string]interface{}{
		"name": "hosted-model",
		"uuid": s.hostedModelUUID,
	}
	args.ControllerCloud = cloud.Cloud{
		Name:      "dummy",
		Type:      "dummy",
		AuthTypes: []cloud.AuthType{cloud.EmptyAuthType},
	}
	args.ControllerCharmRisk = "beta"
	s.bootstrapParams = args
	s.writeBootstrapParamsFile(c)
}

func (s *BootstrapSuite) writeBootstrapParamsFile(c *gc.C) {
	data, err := s.bootstrapParams.Marshal()
	c.Assert(err, jc.ErrorIsNil)
	err = ioutil.WriteFile(s.bootstrapParamsFile, data, 0600)
	c.Assert(err, jc.ErrorIsNil)
}

func nullContext() environs.BootstrapContext {
	ctx, _ := cmd.DefaultContext()
	ctx.Stdin = io.LimitReader(nil, 0)
	ctx.Stdout = ioutil.Discard
	ctx.Stderr = ioutil.Discard
	return modelcmd.BootstrapContext(context.Background(), ctx)
}

type mockDummyEnviron struct {
	environs.Environ
}

func (m *mockDummyEnviron) Instances(ctx envcontext.ProviderCallContext, ids []instance.Id) ([]instances.Instance, error) {
	// ensure that callback is used...
	ctx.InvalidateCredential("considered invalid for the sake of testing")
	return m.Environ.Instances(ctx, ids)
}
