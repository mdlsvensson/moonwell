using Moonwell.Tool;
using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>A map with the physics package builds, and its script loads with the package's code in it; the Sample, without it, carries none of the package.</summary>
    [Collection("Sample")]
    public sealed class PhysicsBuildTests : IDisposable
    {
        private readonly SampleBuilds builds;
        private readonly TestFolders folders = new TestFolders();

        public PhysicsBuildTests(SampleBuilds builds)
        {
            this.builds = builds;
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void map_with_the_package_builds()
        {
            var project = folders.TemporaryCopyOf("Physics");
            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);

            var script = File.ReadAllText(Path.Combine(project, "build", "Physics", "war3map.lua"));
            using var lua = new NLua.Lua();
            lua.DoString("print = function() end");
            lua.DoString("called = {}");
            lua.DoString("setmetatable(_G, { __index = function(_, name) return function() called[#called + 1] = name; return nil end end })");
            lua.DoString(script);
            lua.DoString("main()");
            // The script loads with the package's types in it and main runs under the stubbed natives; the game check covers the rest.
            var called = (string)lua.DoString("return ',' .. table.concat(called, ',') .. ','")[0];
            Assert.Contains(",GetWorldBounds,", called);
            Assert.Contains("Moonwell.Missile", script);
            Assert.Contains("Moonwell.Physics", script);
            Assert.Contains("Firebolt", script);
            Assert.Contains("Knockback", script);
        }

        [Fact]
        public void sample_without_the_package_carries_none_of_it()
        {
            foreach (var script in new[] { builds.Development.Script, builds.Release.Script })
            {
                Assert.DoesNotContain("Missile", script);
                Assert.DoesNotContain("Knockback", script);
            }
        }
    }
}
