using Moonwell.Tool;
using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>A map with the interactions package builds, and gets the table of its tree types in its script; the Sample, without it, carries none of the package.</summary>
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
            var project = CopyWithPackage("Damage", "DamageFixture");

            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);

            var script = File.ReadAllText(Path.Combine(project, "build", "Damage", "war3map.lua"));
            using var lua = new NLua.Lua();
            lua.DoString("print = function() end");
            lua.DoString("setmetatable(_G, { __index = function(_, name) return function() return nil end end })");
            lua.DoString(script);
            Assert.True((bool)lua.DoString($"return MoonwellTrees[{ObjectLua.FourCC("LTlt")}] == true")[0]);
            Assert.True((bool)lua.DoString($"return MoonwellTrees[{ObjectLua.FourCC("LTbr")}] == nil")[0]);

            // The compiled IsTree runs the native world's template on a destructable of each type: always a real boolean.
            lua.DoString("main()");
            lua.DoString("function isTree(id) local d = Moonwell.Destructable(); d.TypeId = Moonwell.ObjectTypes.FourCC(id); return Moonwell.TreeExtensions.IsTree(d) end");
            Assert.True((bool)lua.DoString("return isTree('LTlt') == true")[0]);
            Assert.True((bool)lua.DoString("return isTree('LTbr') == false")[0]);
            lua.DoString("saved = MoonwellTrees; MoonwellTrees = nil");
            Assert.True((bool)lua.DoString("return isTree('LTlt') == false")[0]);
            lua.DoString("MoonwellTrees = saved");
        }

        [Fact]
        public void map_with_the_package_and_no_objects_is_told_it_gets_no_tree_table()
        {
            // The Maths fixture with its one use of Moonwell.Objects taken out; the standard data the table is made from is only in that library.
            var project = CopyWithPackage("Maths", "MathsFixture");
            File.WriteAllText(Path.Combine(project, "Push.cs"), "using Moonwell;\n\nnamespace MathsFixture\n{\n    public sealed class Push : GameComponent\n    {\n    }\n}\n");

            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);

            Assert.Contains("warning: the map uses Moonwell.Interactions without Moonwell.Objects: IsTree needs Moonwell.Objects", output.ToString() + error);
            Assert.DoesNotContain("MoonwellTrees = {", File.ReadAllText(Path.Combine(project, "build", "Maths", "war3map.lua")));
        }

        [Fact]
        public void map_with_the_package_builds()
        {
            var project = folders.TemporaryCopyOf("Interactions");
            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);

            var script = File.ReadAllText(Path.Combine(project, "build", "Interactions", "war3map.lua"));
            using var lua = new NLua.Lua();
            lua.DoString("print = function() end");
            lua.DoString("called = {}");
            lua.DoString("setmetatable(_G, { __index = function(_, name) return function() called[#called + 1] = name; return nil end end })");
            lua.DoString(script);
            lua.DoString("main()");
            // The script loads with the package's types in it and main runs under the stubbed natives; the game check covers the rest.
            var called = (string)lua.DoString("return ',' .. table.concat(called, ',') .. ','")[0];
            Assert.Contains(",GetWorldBounds,", called);
            Assert.Contains("Moonwell.Gizmo", script);
            Assert.Contains("Moonwell.Find", script);
            Assert.Contains("Moonwell.Interaction", script);
        }

        [Fact]
        public void sample_without_the_package_carries_none_of_it()
        {
            foreach (var script in new[] { builds.Development.Script, builds.Release.Script })
            {
                Assert.DoesNotContain("Gizmo", script);
                Assert.DoesNotContain("Moonwell.Find", script); // not plain "Find": the runtime has Array.Find and KnownUnits.Find
                Assert.DoesNotContain("Interaction", script);
                Assert.DoesNotContain("MoonwellTrees", script);
            }
        }

        // A copy of a fixture with the interactions package referenced and IsTree called from a method of its own, so that the
        // compiler keeps the reference.
        private string CopyWithPackage(string fixture, string space)
        {
            var project = folders.TemporaryCopyOf(fixture);
            var projectFile = Path.Combine(project, fixture + ".csproj");
            File.WriteAllText(
                projectFile,
                File.ReadAllText(projectFile).Replace(
                    "  </ItemGroup>\n\n</Project>",
                    "    <ProjectReference Include=\"..\\..\\..\\..\\src\\Moonwell.Interactions\\Moonwell.Interactions.csproj\" />\n  </ItemGroup>\n\n</Project>"));
            File.WriteAllText(
                Path.Combine(project, "Trees.cs"),
                "namespace " + space + "\n{\n    public static class Trees\n    {\n        public static bool Check(Moonwell.Destructable destructable)\n        {\n            return Moonwell.TreeExtensions.IsTree(destructable);\n        }\n    }\n}\n");
            return project;
        }
    }
}
