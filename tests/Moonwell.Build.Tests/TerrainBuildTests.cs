using War3Net.Build.Environment;
using War3Net.Build.Extensions;
using Moonwell.Tool;
using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>A map with the terrain package gets its grid written and installed; a map without it carries none of it.</summary>
    [Collection("Sample")]
    public sealed class TerrainBuildTests : IDisposable
    {
        private readonly SampleBuilds builds;
        private readonly TestFolders folders;

        public TerrainBuildTests(SampleBuilds builds, ITestOutputHelper output)
        {
            this.builds = builds;
            folders = new TestFolders(output);
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void map_with_the_package_installs_the_grid()
        {
            var project = folders.TemporaryCopyOf("Terrain");
            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);

            MapEnvironment environment;
            using (var reader = new BinaryReader(File.OpenRead(Path.Combine(project, "map", "war3map.w3e"))))
            {
                environment = reader.ReadMapEnvironment();
            }

            var generated = Path.Combine(project, "build", "generated");
            var data = File.ReadAllText(Path.Combine(generated, "TerrainData.g.cs"));
            Assert.Contains("public const int Columns = " + (environment.Width + 1) + ";", data);
            Assert.Contains("public const int Rows = " + (environment.Height + 1) + ";", data);

            var main = File.ReadAllText(Path.Combine(generated, "MapMain.g.cs"));
            Assert.Contains("global::Moonwell.Terrain.Install(", main);

            var script = File.ReadAllText(Path.Combine(project, "build", "Terrain", "war3map.lua"));
            using var lua = new NLua.Lua();
            lua.DoString("print = function() end");
            lua.DoString("called = {}");
            lua.DoString("setmetatable(_G, { __index = function(_, name) return function() called[#called + 1] = name; return nil end end })");
            lua.DoString(script);
            lua.DoString("main()");
            // This proves the script loads with the package's code and the grid in it, and that main runs. It does not prove
            // that Install unpacks the grid: the stubs never run the register callback. The game check covers the reads.
            var called = (string)lua.DoString("return ',' .. table.concat(called, ',') .. ','")[0];
            Assert.Contains(",GetWorldBounds,", called);
            Assert.Contains("Terrain.Install", script);
            Assert.Contains("HeightGrid", script);
        }

        [Fact]
        public void dropping_the_package_removes_the_grid()
        {
            var project = folders.TemporaryCopyOf("Maths");
            var generated = Path.Combine(project, "build", "generated");
            Directory.CreateDirectory(generated);
            var stale = Path.Combine(generated, "TerrainData.g.cs");
            File.WriteAllText(stale, "namespace Moonwell.Generated { public static class TerrainData { public const int Columns = 1; } }\n");

            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));

            Assert.True(exitCode == 0, error.ToString() + output);
            Assert.False(File.Exists(stale));
            var script = File.ReadAllText(Path.Combine(project, "build", "Maths", "war3map.lua"));
            Assert.DoesNotContain("TerrainData", script);
        }

        [Fact]
        public void sample_without_the_package_carries_none_of_it()
        {
            var script = builds.Development.Script;
            Assert.DoesNotContain("Terrain.Install", script);
            Assert.DoesNotContain("HeightGrid", script);
            Assert.DoesNotContain("TerrainData", script);
        }
    }
}
