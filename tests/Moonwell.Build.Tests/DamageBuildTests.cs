using Moonwell.Tool;
using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>A map with the damage package installs it from its Main; the Sample, without it, carries none of it.</summary>
    [Collection("Sample")]
    public sealed class DamageBuildTests : IDisposable
    {
        private readonly SampleBuilds builds;
        private readonly TestFolders folders;

        public DamageBuildTests(SampleBuilds builds, ITestOutputHelper output)
        {
            this.builds = builds;
            folders = new TestFolders(output);
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void map_with_the_package_installs_it()
        {
            var project = folders.TemporaryCopyOf("Damage");
            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);

            var main = File.ReadAllText(Path.Combine(project, "build", "generated", "MapMain.g.cs"));
            Assert.Contains("global::Moonwell.DamageSystem.Install();", main);

            var script = File.ReadAllText(Path.Combine(project, "build", "Damage", "war3map.lua"));
            using var lua = new NLua.Lua();
            lua.DoString("print = function() end");
            lua.DoString("called = {}");
            lua.DoString("setmetatable(_G, { __index = function(_, name) return function() called[#called + 1] = name; return nil end end })");
            lua.DoString(script);
            lua.DoString("main()");
            // The script loads (a Footman subclass implementing IDamageTarget included) and main runs; the setup that
            // installs the package waits for a timer the stubbed game never fires, so the game check covers the rest.
            var called = (string)lua.DoString("return ',' .. table.concat(called, ',') .. ','")[0];
            Assert.Contains(",GetWorldBounds,", called);
            Assert.Contains("DamageSystem.Install()", script);
            Assert.Contains("EVENT_PLAYER_UNIT_DAMAGING", script);
        }

        [Fact]
        public void sample_without_the_package_carries_none_of_it()
        {
            var script = builds.Development.Script;
            Assert.DoesNotContain("DamageSystem", script);
            Assert.DoesNotContain("Moonwell.Hit", script);
            Assert.DoesNotContain("EVENT_PLAYER_UNIT_DAMAGING", script);
            var main = File.ReadAllText(Path.Combine(builds.Folder, "build", "generated", "MapMain.g.cs"));
            Assert.DoesNotContain("DamageSystem", main);
        }
    }
}
