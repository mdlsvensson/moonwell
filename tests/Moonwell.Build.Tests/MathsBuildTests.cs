using Moonwell.Tool;
using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>A map with the maths package builds and its script loads; the Sample, without it, carries none of it.</summary>
    [Collection("Sample")]
    public sealed class MathsBuildTests : IDisposable
    {
        private readonly SampleBuilds builds;
        private readonly TestFolders folders;

        public MathsBuildTests(SampleBuilds builds, ITestOutputHelper output)
        {
            this.builds = builds;
            folders = new TestFolders(output);
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void map_with_the_package_builds_and_loads()
        {
            var project = folders.TemporaryCopyOf("Maths");
            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);

            var script = File.ReadAllText(Path.Combine(project, "build", "Maths", "war3map.lua"));
            using var lua = new NLua.Lua();
            lua.DoString("print = function() end");
            lua.DoString("called = {}");
            lua.DoString("setmetatable(_G, { __index = function(_, name) return function() called[#called + 1] = name; return nil end end })");
            lua.DoString(script);
            lua.DoString("main()");
            // The script loads with the package's types in it and main runs; the game check covers the rest.
            var called = (string)lua.DoString("return ',' .. table.concat(called, ',') .. ','")[0];
            Assert.Contains(",GetWorldBounds,", called);
            Assert.Contains("Moonwell.Vector2", script);
            Assert.Contains("Moonwell.Polygon", script);
            Assert.Contains("ActorMaths", script);
        }

        [Fact]
        public void sample_without_the_package_carries_none_of_it()
        {
            var script = builds.Development.Script;
            Assert.DoesNotContain("Moonwell.Vector2", script);
            Assert.DoesNotContain("Moonwell.Polygon", script);
            Assert.DoesNotContain("ActorMaths", script);
        }
    }
}
