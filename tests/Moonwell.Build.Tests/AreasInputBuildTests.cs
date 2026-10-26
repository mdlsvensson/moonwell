using Moonwell.Tool;
using Xunit;

namespace Moonwell.Build.Tests
{
    /// <summary>A map with the areas or input package gets its regions and cameras written and installed; a map without them carries none of it.</summary>
    [Collection("Sample")]
    public sealed class AreasInputBuildTests : IDisposable
    {
        private readonly SampleBuilds builds;
        private readonly TestFolders folders = new TestFolders();

        public AreasInputBuildTests(SampleBuilds builds)
        {
            this.builds = builds;
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void map_with_areas_installs_regions()
        {
            var project = folders.TemporaryCopyOf("Areas");
            Build(project);

            var generated = Path.Combine(project, "build", "generated");
            var data = File.ReadAllText(Path.Combine(generated, "AreaData.g.cs"));
            Assert.Contains("\"Shop\"", data);
            Assert.Contains("\"Arena\"", data);
            Assert.False(File.Exists(Path.Combine(generated, "InputData.g.cs")));

            var main = File.ReadAllText(Path.Combine(generated, "MapMain.g.cs"));
            Assert.Contains("global::Moonwell.AreaSystem.Install(", main);
            Assert.DoesNotContain("InputSystem", main);

            var script = File.ReadAllText(Path.Combine(project, "build", "Areas", "war3map.lua"));
            using var lua = new NLua.Lua();
            lua.DoString("print = function() end");
            lua.DoString("setmetatable(_G, { __index = function(_, name) return function() return nil end end })");
            lua.DoString(script);
            lua.DoString("main()");

            // This proves the script loads with the package's code and the regions in it, and that main runs. It does not prove
            // that Install unpacks them: the stubs never run the register callback. The game check covers the reads.
            Assert.Contains("AreaSystem.Install", script);
            Assert.Contains("AreaData", script);
            Assert.DoesNotContain("InputSystem", script);
        }

        [Fact]
        public void unknown_region_fails()
        {
            var project = folders.TemporaryCopyOf("Areas");
            var file = Path.Combine(project, "ShopFloor.cs");
            var text = File.ReadAllText(file);
            var changed = text.Replace("AreaShape.Region(\"Shop\")", "AreaShape.Region(\"Bank\")");
            Assert.NotEqual(text, changed);
            File.WriteAllText(file, changed);
            var line = changed.Split('\n').ToList().FindIndex(one => one.Contains("\"Bank\"")) + 1;

            var error = BuildFails(project);

            Assert.Contains($"ShopFloor.cs:{line}: the map has no World Editor region named 'Bank'. Its regions: Shop, Arena.", error);
        }

        [Fact]
        public void map_with_mouse_listens()
        {
            var project = folders.TemporaryCopyOf("Input");
            Build(project);

            var generated = Path.Combine(project, "build", "generated");
            var data = File.ReadAllText(Path.Combine(generated, "InputData.g.cs"));
            Assert.Contains("public const bool Mouse = true;", data);
            Assert.Contains("\"Intro\"", data);
            Assert.False(File.Exists(Path.Combine(generated, "AreaData.g.cs")));

            var main = File.ReadAllText(Path.Combine(generated, "MapMain.g.cs"));
            Assert.Contains("global::Moonwell.InputSystem.Install(", main);
            Assert.DoesNotContain("AreaSystem", main);

            var script = File.ReadAllText(Path.Combine(project, "build", "Input", "war3map.lua"));
            using var lua = new NLua.Lua();
            lua.DoString("print = function() end");
            lua.DoString("setmetatable(_G, { __index = function(_, name) return function() return nil end end })");
            lua.DoString(script);
            lua.DoString("main()");
            Assert.Contains("InputSystem.Install", script);
            Assert.DoesNotContain("AreaSystem", script);
        }

        [Fact]
        public void camera_only_map_does_not_listen()
        {
            var project = folders.TemporaryCopyOf("Input");
            File.Delete(Path.Combine(project, "Aim.cs"));
            File.WriteAllText(
                Path.Combine(project, "Tour.cs"),
                "using Moonwell;\n\nnamespace InputFixture\n{\n    public sealed class Tour : GameComponent\n    {\n        public override void OnGameStart()\n        {\n            Player.One.Camera().Apply(CameraSetup.Named(\"Intro\"), 2f);\n        }\n    }\n}\n");

            Build(project);

            var data = File.ReadAllText(Path.Combine(project, "build", "generated", "InputData.g.cs"));
            Assert.Contains("public const bool Mouse = false;", data);
            Assert.Contains("\"Intro\"", data);
        }

        [Fact]
        public void unknown_camera_fails()
        {
            var project = folders.TemporaryCopyOf("Input");
            var file = Path.Combine(project, "Tour.cs");
            var text = File.ReadAllText(file);
            var changed = text.Replace("CameraSetup.Named(\"Intro\")", "CameraSetup.Named(\"Outro\")");
            Assert.NotEqual(text, changed);
            File.WriteAllText(file, changed);
            var line = changed.Split('\n').ToList().FindIndex(one => one.Contains("\"Outro\"")) + 1;

            var error = BuildFails(project);

            Assert.Contains($"Tour.cs:{line}: the map has no World Editor camera named 'Outro'. Its cameras: Intro.", error);
        }

        [Fact]
        public void dropping_packages_removes_data()
        {
            // An areas map that also references the input package: both files are written, and both are gone once it references neither.
            var project = folders.TemporaryCopyOf("Areas");
            var csproj = Path.Combine(project, "Areas.csproj");
            var references = File.ReadAllText(csproj);
            const string Input = "<ProjectReference Include=\"..\\..\\..\\..\\src\\Moonwell.Input\\Moonwell.Input.csproj\" />";
            const string Areas = "<ProjectReference Include=\"..\\..\\..\\..\\src\\Moonwell.Areas\\Moonwell.Areas.csproj\" />";
            Assert.Contains(Areas, references);
            File.WriteAllText(csproj, references.Replace(Areas, Areas + "\n    " + Input));
            File.WriteAllText(
                Path.Combine(project, "Look.cs"),
                "using Moonwell;\n\nnamespace AreasFixture\n{\n    public sealed class Look : GameComponent\n    {\n        public override void OnGameStart()\n        {\n            Player.One.Camera().Reset();\n        }\n    }\n}\n");
            var map = File.ReadAllText(Path.Combine(project, "Map.cs"));
            File.WriteAllText(Path.Combine(project, "Map.cs"), map.Replace("Components.Add(new ShopFloor());", "Components.Add(new ShopFloor());\n            Components.Add(new Look());"));
            Build(project);

            var generated = Path.Combine(project, "build", "generated");
            Assert.True(File.Exists(Path.Combine(generated, "AreaData.g.cs")));
            Assert.True(File.Exists(Path.Combine(generated, "InputData.g.cs")));

            File.WriteAllText(csproj, references.Replace(Areas, "<!-- no packages -->"));
            File.Delete(Path.Combine(project, "ShopFloor.cs"));
            File.Delete(Path.Combine(project, "Look.cs"));
            File.WriteAllText(Path.Combine(project, "Map.cs"), map.Replace("Components.Add(new ShopFloor());", string.Empty));
            Build(project);

            Assert.False(File.Exists(Path.Combine(generated, "AreaData.g.cs")));
            Assert.False(File.Exists(Path.Combine(generated, "InputData.g.cs")));
            var script = File.ReadAllText(Path.Combine(project, "build", "Areas", "war3map.lua"));
            Assert.DoesNotContain("AreaData", script);
            Assert.DoesNotContain("InputData", script);
        }

        [Fact]
        public void sample_without_packages_carries_none()
        {
            var script = builds.Development.Script;
            Assert.DoesNotContain("AreaSystem", script);
            Assert.DoesNotContain("InputSystem", script);
            Assert.DoesNotContain("AreaData", script);
            Assert.DoesNotContain("InputData", script);
        }

        private static void Build(string project)
        {
            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 0, error.ToString() + output);
        }

        private static string BuildFails(string project)
        {
            var output = new StringWriter();
            var error = new StringWriter();
            var exitCode = Cli.Run(new[] { "build", project }, output, error, (_, _) => throw new InvalidOperationException("build starts nothing"));
            Assert.True(exitCode == 1, "the build should fail: " + output);
            return error.ToString();
        }
    }
}
