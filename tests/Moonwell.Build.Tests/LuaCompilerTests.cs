using War3Net.Build.Extensions;
using War3Net.Build.Info;
using Xunit;

namespace Moonwell.Build.Tests
{
    [Collection("Sample")]
    public sealed class LuaCompilerTests : IDisposable
    {
        private readonly SampleBuilds builds;
        private readonly TestFolders folders = new TestFolders();
        private readonly List<string> temporaryFolders = new List<string>();

        public LuaCompilerTests(SampleBuilds builds)
        {
            this.builds = builds;
        }

        [Fact]
        public void script_has_main_config_and_entry()
        {
            var script = builds.Development.Script;

            Assert.Contains("function main(", script);
            Assert.Contains("function config(", script);
            Assert.Contains("Moonwell.Generated.MapMain.Main()", script);
            using var lua = new NLua.Lua();
            lua.DoString("print = function() end");
            lua.DoString("called = {}");
            lua.DoString("setmetatable(_G, { __index = function(_, name) return function() called[#called + 1] = name; return nil end end })");
            lua.DoString(script);
            Assert.True((bool)lua.DoString("return rawget(_G, 'main') ~= nil and rawget(_G, 'config') ~= nil")[0]);

            // main runs MapMain.Main, which constructs the map and starts the runtime (which asks the game for the world's bounds).
            lua.DoString("main()");
            var called = (string)lua.DoString("return ',' .. table.concat(called, ',') .. ','")[0];
            Assert.Contains(",InitBlizzard,", called);
            Assert.Contains(",GetWorldBounds,", called);
        }

        [Fact]
        public void development_script_keeps_debug()
        {
            var script = builds.Development.Script;

            Assert.Contains("sample: started (development)", script);
            Assert.DoesNotContain("sample: started (release)", script);
        }

        [Fact]
        public void release_script_drops_debug()
        {
            var script = builds.Release.Script;

            Assert.Contains("sample: started (release)", script);
            Assert.DoesNotContain("sample: started (development)", script);
        }

        [Fact]
        public void development_script_has_line_table()
        {
            AssertLineTable(builds.Development.Script);
        }

        [Fact]
        public void release_script_has_line_table()
        {
            AssertLineTable(builds.Release.Script);
        }

        private static void AssertLineTable(string script)
        {
            Assert.Contains("MoonwellLines = { files = { \"Hello.cs\", \"Map.cs\", \"Objects.cs\" }, map = \"", script);
            Assert.DoesNotContain(script.Split('\n'), line => line.TrimStart().StartsWith("--@mw ", StringComparison.Ordinal));
        }

        [Fact]
        public void jass_source_map_becomes_lua()
        {
            var folder = folders.TemporaryCopyOfSample();
            // Compiled without the objects' stubs, so the copy declares no object.
            File.Delete(Path.Combine(folder, "Objects.cs"));
            var infoPath = Path.Combine(folder, "map", "war3map.w3i");
            MapInfo info;
            using (var reader = new BinaryReader(File.OpenRead(infoPath)))
            {
                info = reader.ReadMapInfo();
            }

            info.ScriptLanguage = ScriptLanguage.Jass;
            using (var writer = new BinaryWriter(File.Create(infoPath)))
            {
                writer.Write(info);
            }

            using (var reader = new BinaryReader(File.OpenRead(infoPath)))
            {
                Assert.Equal(ScriptLanguage.Jass, reader.ReadMapInfo().ScriptLanguage);
            }

            var project = MapProject.Find(folder);
            var assembly = DotnetBuild.Run(project, false, TextWriter.Null);
            var settings = MapSettingsReader.Read(assembly, project);

            var map = SourceMap.Open(project, settings, TextWriter.Null);

            Assert.Equal(ScriptLanguage.Lua, map.Info!.ScriptLanguage);
            Assert.Equal("Sample map", map.Info.MapName);
            Assert.Equal("Moonwell", map.Info.MapAuthor);
            Assert.Equal("Built by the tests.", map.Info.MapDescription);
            Assert.Equal("Any", map.Info.RecommendedPlayers);
            EntryWriter.Write(project, settings);
            Assert.Contains("function main(", LuaCompiler.Compile(project, map, assembly, false));
        }

        [Fact]
        public void unused_map_file_is_not_parsed()
        {
            var folder = folders.TemporaryCopyOfSample();
            var garbage = new byte[] { 7, 0, 0, 0, 9, 9, 9, 9, 9, 9, 9 };
            File.WriteAllBytes(Path.Combine(folder, "map", "war3map.mmp"), garbage);

            var archive = MapBuild.Run(folder, new BuildOptions(), TextWriter.Null);

            Assert.True(File.Exists(archive));
            Assert.Equal(garbage, File.ReadAllBytes(Path.Combine(folder, "build", "Sample", "war3map.mmp")));
        }

        [Fact]
        public void unreadable_map_file_is_reported()
        {
            var folder = folders.TemporaryCopyOfSample();
            File.WriteAllBytes(Path.Combine(folder, "map", "war3map.w3i"), new byte[] { 1, 2, 3 });

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(folder, new BuildOptions(), TextWriter.Null));

            Assert.StartsWith(@"cannot read the World Editor map in map\: ", error.Message);
        }

        [Fact]
        public void csharplua_error_is_a_build_error()
        {
            var folder = folders.TemporaryCopyOfSample();
            File.WriteAllText(
                Path.Combine(folder, "Bad.cs"),
                "namespace Sample\n"
                + "{\n"
                + "    public static class Bad\n"
                + "    {\n"
                + "        public static void Call(dynamic target)\n"
                + "        {\n"
                + "            target.Foo(x: 1);\n"
                + "        }\n"
                + "    }\n"
                + "}\n");

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(folder, new BuildOptions(), TextWriter.Null));

            Assert.Contains("named argument is not support at dynamic", error.Message);
            Assert.DoesNotContain("unexpected error", error.Message);
            Assert.DoesNotContain("marked", error.Message);
        }

        [Fact]
        public void map_without_terrain_is_an_error()
        {
            var folder = folders.TemporaryCopyOfSample();
            File.Delete(Path.Combine(folder, "map", "war3map.w3e"));

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(folder, new BuildOptions(), TextWriter.Null));

            Assert.Equal(@"the script has no main: map\war3map.w3e is missing (save the map in World Editor)", error.Message);
        }

        [Fact]
        public void whole_number_floats_reach_lua_as_floats()
        {
            // Hello.cs passes 0f, 0f; the player number stays an integer (C18).
            Assert.Contains("DisplayTextToPlayer(Player(0), 0.0, 0.0, \"sample: started (development)\")", builds.Development.Script);
            Assert.Contains("DisplayTextToPlayer(Player(0), 0.0, 0.0, \"sample: started (release)\")", builds.Release.Script);

            // Hello.Footman leaves out Player.Print's seconds and Unit.Create's facing, and reads Unit.DefaultFacing: all three
            // come from Moonwell's library, so the build resolved it.
            foreach (var script in new[] { builds.Development.Script, builds.Release.Script })
            {
                Assert.Contains("player:Print(\"sample: a footman, facing \" .. 270.0, -1.0)", script);
                Assert.Contains("Moonwell.Unit.Create(Moonwell.ObjectTypes.FourCC(\"hfoo\"), player, 0.0, 0.0, 270.0)", script);
            }
        }

        [Fact]
        public void name_the_float_rewrite_cannot_resolve_is_a_build_error()
        {
            var folder = folders.TemporaryCopyOfSample();
            File.Delete(Path.Combine(folder, "Objects.cs"));
            File.WriteAllText(
                Path.Combine(folder, "Unread.cs"),
                "namespace Sample\n"
                + "{\n"
                + "    public static class Unread\n"
                + "    {\n"
                + "        public static object Keep(Moonwell.Objects.ObjectIdAttribute id)\n"
                + "        {\n"
                + "            return id;\n"
                + "        }\n"
                + "    }\n"
                + "}\n");
            var project = MapProject.Find(folder);
            var assembly = DotnetBuild.Run(project, false, TextWriter.Null);
            var settings = MapSettingsReader.Read(assembly, project);
            var map = SourceMap.Open(project, settings, TextWriter.Null);
            EntryWriter.Write(project, settings);

            // CSharp.lua compiles Moonwell.Objects from source; the float rewrite reads the libraries beside the assembly.
            File.Delete(Path.Combine(Path.GetDirectoryName(assembly)!, "Moonwell.Objects.dll"));
            var error = Assert.Throws<BuildException>(() => LuaCompiler.Compile(project, map, assembly, false));

            Assert.StartsWith("the build could not resolve these names to write whole-number floats for Lua (C18):", error.Message);
            Assert.Contains(Path.Combine(folder, "Unread.cs"), error.Message);
            Assert.DoesNotContain("marked", error.Message);
        }

        [Fact]
        public void float_constants_are_written_with_a_point()
        {
            var source = string.Join("\n", new[]
            {
                "public class Rewritten",
                "{",
                "    private const float Four = 4f;",
                "    private const float Negative = -4f;",
                "    public float Field = 3f;",
                "    public Rewritten(float scale = 1f) { }",
                "    public static float Facing(float facing = 270f, int count = 2, double @base = 5d) { return facing; }",
                "    public static void Run(float a)",
                "    {",
                "        float folded = 16777216f + 1f;",
                "        float constant = Four;",
                "        float cast = (float)7;",
                "        float minus = -5f;",
                "        float joined = a-Negative;",
                "        float stored = 2;",
                "        double d = 5d;",
                "        a += 1f;",
                "        a *= 2;",
                "        float lines = 1f +",
                "            2f;",
                "        Facing();",
                "        Facing(90f, 3);",
                "        var made = new Rewritten { Field = 1.5f };",
                "        float pointed = 2.0f + 1.0f;",
                "        float half = 2.0f + 0.5f;",
                "        float exponent = 1e3f;",
                "        float signed = -(5.0f);",
                "        int whole = 7 / 2;",
                "        long big = 5;",
                "        string name = nameof(Four);",
                "    }",
                "}",
            });
            var expected = string.Join("\n", new[]
            {
                "public class Rewritten",
                "{",
                "    private const float Four = 4.0f;",
                "    private const float Negative = -4.0f;",
                "    public float Field = 3.0f;",
                "    public Rewritten(float scale = 1f) { }",
                "    public static float Facing(float facing = 270f, int count = 2, double @base = 5d) { return facing; }",
                "    public static void Run(float a)",
                "    {",
                "        float folded = 16777216.0f;",
                "        float constant = 4.0f;",
                "        float cast = 7.0f;",
                "        float minus = -5.0f;",
                "        float joined = a-(-4.0f);",
                "        float stored = 2.0f;",
                "        double d = 5.0;",
                "        a += 1.0f;",
                "        a *= 2.0f;",
                "        float lines = 3.0f",
                ";",
                "        Facing(facing: 270.0f, @base: 5.0);",
                "        Facing(90.0f, 3, @base: 5.0);",
                "        var made = new Rewritten(scale: 1.0f) { Field = 1.5f };",
                "        float pointed = 3.0f;",
                "        float half = 2.0f + 0.5f;",
                "        float exponent = 1e3f;",
                "        float signed = -(5.0f);",
                "        int whole = 7 / 2;",
                "        long big = 5;",
                "        string name = nameof(Four);",
                "    }",
                "}",
            });
            var folder = Path.Combine(Path.GetTempPath(), "moonwell-floats-" + Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(folder);
            temporaryFolders.Add(folder);
            File.WriteAllText(Path.Combine(folder, "Rewritten.cs"), source);

            var errors = FloatLiterals.Rewrite(folder, new[] { "Rewritten.cs" }, "Rewritten", Array.Empty<string>(), Array.Empty<string>());

            Assert.Empty(errors);
            Assert.Equal(expected, File.ReadAllText(Path.Combine(folder, "Rewritten.cs")));
        }

        [Fact]
        public void a_left_out_default_of_a_base_constructor_is_written_in()
        {
            // : base(...) and a primary constructor's base call the base constructor as a call does (C18).
            var source = string.Join("\n", new[]
            {
                "public class Shape",
                "{",
                "    public Shape(int count, float scale = 2f) { }",
                "}",
                "public class Square : Shape",
                "{",
                "    public Square() : base(1) { }",
                "    public Square(int count) : this() { }",
                "}",
                "public class Circle() : Shape(1);",
                "public class Dot : Shape",
                "{",
                "    public Dot(float scale = 3f) : base(1, scale) { }",
                "    public Dot(int count) : this() { }",
                "}",
            });
            var expected = source
                .Replace(": base(1) { }", ": base(1, scale: 2.0f) { }")
                .Replace("Shape(1);", "Shape(1, scale: 2.0f);")
                .Replace("public Dot(int count) : this()", "public Dot(int count) : this(scale: 3.0f)");
            var folder = Path.Combine(Path.GetTempPath(), "moonwell-floats-" + Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(folder);
            temporaryFolders.Add(folder);
            File.WriteAllText(Path.Combine(folder, "Bases.cs"), source);

            var errors = FloatLiterals.Rewrite(folder, new[] { "Bases.cs" }, "Bases", Array.Empty<string>(), Array.Empty<string>());

            Assert.Empty(errors);
            Assert.Equal(expected, File.ReadAllText(Path.Combine(folder, "Bases.cs")));
        }

        [Fact]
        public void missing_libraries_are_an_error()
        {
            var empty = Path.Combine(Path.GetTempPath(), "moonwell-empty-" + Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(empty);
            temporaryFolders.Add(empty);
            var assembly = Path.Combine(empty, "Map.dll");

            var error = Assert.Throws<BuildException>(() => ScriptLibraries.Extract(assembly, Path.Combine(empty, "lua")));

            Assert.Equal($"Moonwell.Natives.dll is not next to {assembly}", error.Message);
        }

        public void Dispose()
        {
            folders.Dispose();
            foreach (var folder in temporaryFolders)
            {
                if (Directory.Exists(folder))
                {
                    Directory.Delete(folder, true);
                }
            }
        }
    }
}
