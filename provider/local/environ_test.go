// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package local_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	gc "launchpad.net/gocheck"

	"launchpad.net/juju-core/agent"
	coreCloudinit "launchpad.net/juju-core/cloudinit"
	"launchpad.net/juju-core/constraints"
	"launchpad.net/juju-core/environs"
	"launchpad.net/juju-core/environs/cloudinit"
	"launchpad.net/juju-core/environs/config"
	"launchpad.net/juju-core/environs/jujutest"
	envtesting "launchpad.net/juju-core/environs/testing"
	"launchpad.net/juju-core/environs/tools"
	"launchpad.net/juju-core/provider/local"
	"launchpad.net/juju-core/state"
	coretesting "launchpad.net/juju-core/testing"
	jc "launchpad.net/juju-core/testing/checkers"
)

type environSuite struct {
	baseProviderSuite
	envtesting.ToolsFixture
}

var _ = gc.Suite(&environSuite{})

func (s *environSuite) SetUpTest(c *gc.C) {
	s.baseProviderSuite.SetUpTest(c)
	s.ToolsFixture.SetUpTest(c)
}

func (s *environSuite) TearDownTest(c *gc.C) {
	s.ToolsFixture.TearDownTest(c)
	s.baseProviderSuite.TearDownTest(c)
}

func (*environSuite) TestOpenFailsWithProtectedDirectories(c *gc.C) {
	testConfig := minimalConfig(c)
	testConfig, err := testConfig.Apply(map[string]interface{}{
		"root-dir": "/usr/lib/juju",
	})
	c.Assert(err, gc.IsNil)

	environ, err := local.Provider.Open(testConfig)
	c.Assert(err, gc.ErrorMatches, "mkdir .* permission denied")
	c.Assert(environ, gc.IsNil)
}

func (s *environSuite) TestNameAndStorage(c *gc.C) {
	testConfig := minimalConfig(c)
	environ, err := local.Provider.Open(testConfig)
	c.Assert(err, gc.IsNil)
	c.Assert(environ.Name(), gc.Equals, "test")
	c.Assert(environ.Storage(), gc.NotNil)
}

func (s *environSuite) TestGetToolsMetadataSources(c *gc.C) {
	testConfig := minimalConfig(c)
	environ, err := local.Provider.Open(testConfig)
	c.Assert(err, gc.IsNil)
	sources, err := tools.GetMetadataSources(environ)
	c.Assert(err, gc.IsNil)
	c.Assert(len(sources), gc.Equals, 1)
	url, err := sources[0].URL("")
	c.Assert(err, gc.IsNil)
	c.Assert(strings.Contains(url, "/tools"), jc.IsTrue)
}

type localJujuTestSuite struct {
	baseProviderSuite
	jujutest.Tests
	restoreRootCheck   func()
	oldUpstartLocation string
	oldPath            string
	testPath           string
	dbServiceName      string
}

func (s *localJujuTestSuite) SetUpTest(c *gc.C) {
	s.baseProviderSuite.SetUpTest(c)
	// Construct the directories first.
	err := local.CreateDirs(c, minimalConfig(c))
	c.Assert(err, gc.IsNil)
	s.oldPath = os.Getenv("PATH")
	s.testPath = c.MkDir()
	s.PatchEnvPathPrepend(s.testPath)

	// Add in an admin secret
	s.Tests.TestConfig["admin-secret"] = "sekrit"
	s.restoreRootCheck = local.SetRootCheckFunction(func() bool { return false })
	s.Tests.SetUpTest(c)

	cfg, err := config.New(config.NoDefaults, s.TestConfig)
	c.Assert(err, gc.IsNil)
	s.dbServiceName = "juju-db-" + local.ConfigNamespace(cfg)
}

func (s *localJujuTestSuite) TearDownTest(c *gc.C) {
	s.Tests.TearDownTest(c)
	s.restoreRootCheck()
	s.baseProviderSuite.TearDownTest(c)
}

func (s *localJujuTestSuite) MakeTool(c *gc.C, name, script string) {
	path := filepath.Join(s.testPath, name)
	script = "#!/bin/bash\n" + script
	err := ioutil.WriteFile(path, []byte(script), 0755)
	c.Assert(err, gc.IsNil)
}

func (s *localJujuTestSuite) StoppedStatus(c *gc.C) {
	s.MakeTool(c, "status", `echo "some-service stop/waiting"`)
}

func (s *localJujuTestSuite) RunningStatus(c *gc.C) {
	s.MakeTool(c, "status", `echo "some-service start/running, process 123"`)
}

var _ = gc.Suite(&localJujuTestSuite{
	Tests: jujutest.Tests{
		TestConfig: minimalConfigValues(),
	},
})

func (s *localJujuTestSuite) TestBootstrap(c *gc.C) {
	s.PatchValue(local.FinishBootstrap, func(mcfg *cloudinit.MachineConfig, cloudcfg *coreCloudinit.Config, ctx environs.BootstrapContext) error {
		c.Assert(cloudcfg.AptUpdate(), jc.IsFalse)
		c.Assert(cloudcfg.AptUpgrade(), jc.IsFalse)
		c.Assert(cloudcfg.Packages(), gc.HasLen, 0)
		c.Assert(mcfg.AgentEnvironment, gc.Not(gc.IsNil))
		// local does not allow machine-0 to host units
		bootstrapJobs, err := agent.UnmarshalBootstrapJobs(mcfg.AgentEnvironment[agent.BootstrapJobs])
		c.Assert(err, gc.IsNil)
		c.Assert(bootstrapJobs, gc.DeepEquals, []state.MachineJob{state.JobManageEnviron})
		return nil
	})
	testConfig := minimalConfig(c)
	ctx := coretesting.Context(c)
	environ, err := local.Provider.Prepare(ctx, testConfig)
	c.Assert(err, gc.IsNil)
	envtesting.UploadFakeTools(c, environ.Storage())
	defer environ.Storage().RemoveAll()
	err = environ.Bootstrap(ctx, constraints.Value{})
	c.Assert(err, gc.IsNil)
}

func (s *localJujuTestSuite) TestStartStop(c *gc.C) {
	c.Skip("StartInstance not implemented yet.")
}
