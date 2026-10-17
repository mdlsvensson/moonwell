using War3Net.Build.Extensions;
using War3Net.Build.Info;
using Xunit;

namespace Moonwell.Build.Tests
{
    [Collection("Sample")]
    public sealed class LuaCompilerTests : IDisposable
    {
        private readonly List<string> temporaryFolders = new List<string>();

        [Fact]
        public void script_has_main_config_and_entry()
        {
            var script = CompileScript(Fixture("Sample"), false);

            Assert.Contains("function main(", script);
            Assert.Contains("function config(", script);
            Assert.Contains("Moonwell.Generated.MapMain.Main()", script);
            using var lua = new NLua.Lua();
            lua.DoString("print = function() end");
            lua.DoString("setmetatable(_G, { __index = function(_, name) return function() return nil end end })");
            lua.DoString(script);
            Assert.True((bool)lua.DoString("return rawget(_G, 'main') ~= nil and rawget(_G, 'config') ~= nil")[0]);
        }

        [Fact]
        public void development_script_keeps_debug()
        {
            var script = CompileScript(Fixture("Sample"), false);

            Assert.Contains("sample: started (development)", script);
            Assert.DoesNotContain("sample: started (release)", script);
        }

        [Fact]
        public void release_script_drops_debug()
        {
            var script = CompileScript(Fixture("Sample"), true);

            Assert.Contains("sample: started (release)", script);
            Assert.DoesNotContain("sample: started (development)", script);
        }

        [Fact]
        public void jass_source_map_becomes_lua()
        {
            var folder = TemporaryCopyOfSample();
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

            var map = SourceMap.Open(project, settings);

            Assert.Equal(ScriptLanguage.Lua, map.Info!.ScriptLanguage);
            Assert.Equal("Sample map", map.Info.MapName);
            Assert.Equal("Moonwell", map.Info.MapAuthor);
            Assert.Equal("Built by the tests.", map.Info.MapDescription);
            Assert.Equal("Any", map.Info.RecommendedPlayers);
            EntryWriter.Write(project, settings);
            Assert.Contains("function main(", LuaCompiler.Compile(project, map, assembly, false));
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
            foreach (var folder in temporaryFolders)
            {
                if (Directory.Exists(folder))
                {
                    Directory.Delete(folder, true);
                }
            }
        }

        private static string Fixture(string name)
        {
            return Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "Fixtures", name));
        }

        private static string CompileScript(string folder, bool release)
        {
            var project = MapProject.Find(folder);
            var assembly = DotnetBuild.Run(project, release, TextWriter.Null);
            var settings = MapSettingsReader.Read(assembly, project);
            var map = SourceMap.Open(project, settings);
            EntryWriter.Write(project, settings);
            return LuaCompiler.Compile(project, map, assembly, release);
        }

        private string TemporaryCopyOfSample()
        {
            var folder = Path.Combine(Fixture("."), "Temp-" + Guid.NewGuid().ToString("N"));
            temporaryFolders.Add(folder);
            CopyFolder(Fixture("Sample"), folder);
            return folder;
        }

        private static void CopyFolder(string from, string to)
        {
            Directory.CreateDirectory(to);
            foreach (var file in Directory.GetFiles(from))
            {
                File.Copy(file, Path.Combine(to, Path.GetFileName(file)));
            }

            foreach (var directory in Directory.GetDirectories(from))
            {
                var name = Path.GetFileName(directory);
                if (name is "bin" or "obj" or "build")
                {
                    continue;
                }

                CopyFolder(directory, Path.Combine(to, name));
            }
        }
    }
}
