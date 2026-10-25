using Moonwell.Tool;
using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>A map with the interactions package gets the table of its tree types in its script; the Sample, without it, gets none.</summary>
    [Collection("Sample")]
    public sealed class InteractionsBuildTests : IDisposable
    {
        private readonly SampleBuilds builds;
        private readonly TestFolders folders = new TestFolders();

        public InteractionsBuildTests(SampleBuilds builds)
        {
            this.builds = builds;
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void map_with_the_package_gets_the_tree_table()
        {
            // The Damage fixture, which uses Moonwell.Objects, with the package referenced and IsTree called from a method.
            var project = folders.TemporaryCopyOf("Damage");
            var projectFile = Path.Combine(project, "Damage.csproj");
            File.WriteAllText(
                projectFile,
                File.ReadAllText(projectFile).Replace(
                    "  </ItemGroup>\n\n</Project>",
                    "    <ProjectReference Include=\"..\\..\\..\\..\\src\\Moonwell.Interactions\\Moonwell.Interactions.csproj\" />\n  </ItemGroup>\n\n</Project>"));
            File.WriteAllText(
                Path.Combine(project, "Trees.cs"),
                "namespace DamageFixture\n{\n    public static class Trees\n    {\n        public static bool Check(Moonwell.Destructable destructable)\n        {\n            return Moonwell.TreeExtensions.IsTree(destructable);\n        }\n    }\n}\n");

            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);

            var script = File.ReadAllText(Path.Combine(project, "build", "Damage", "war3map.lua"));
            using var lua = new NLua.Lua();
            lua.DoString("print = function() end");
            lua.DoString("setmetatable(_G, { __index = function(_, name) return function() return nil end end })");
            lua.DoString(script);
            // The script is only loaded: the table is a global the native world's template reads.
            Assert.True((bool)lua.DoString($"return MoonwellTrees[{ObjectLua.FourCC("LTlt")}] == true")[0]);
            Assert.True((bool)lua.DoString($"return MoonwellTrees[{ObjectLua.FourCC("LTbr")}] == nil")[0]);
        }

        [Fact]
        public void sample_without_the_package_carries_no_tree_table()
        {
            Assert.DoesNotContain("MoonwellTrees", builds.Development.Script);
            Assert.DoesNotContain("MoonwellTrees", builds.Release.Script);
        }
    }
}
