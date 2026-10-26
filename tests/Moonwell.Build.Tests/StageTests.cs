using System.Security.Cryptography;
using War3Net.Build.Extensions;
using War3Net.Build.Info;
using War3Net.IO.Mpq;
using Xunit;

namespace Moonwell.Build.Tests
{
    [Collection("Sample")]
    public sealed class StageTests : IDisposable
    {
        private static readonly string[] DroppedFiles = { "war3map.j" };

        private static readonly Lazy<(MapProject Project, War3Net.Build.Map Map)> CompiledSample =
            new Lazy<(MapProject, War3Net.Build.Map)>(() =>
            {
                var project = MapProject.Find(TestFolders.Fixture("Sample"));
                var assembly = DotnetBuild.Run(project, false, TextWriter.Null);
                var settings = MapSettingsReader.Read(assembly, project);
                var map = SourceMap.Open(project, settings, TextWriter.Null);
                EntryWriter.Write(project, settings);
                LuaCompiler.Compile(project, map, assembly, false);
                return (project, map);
            });

        private readonly TestFolders folders;

        public StageTests(ITestOutputHelper output)
        {
            folders = new TestFolders(output);
        }

        [Fact]
        public void source_map_is_untouched()
        {
            var (project, map) = CompiledSample.Value;
            var before = Hashes(project.MapFolder).Concat(Hashes(project.AssetsFolder)).ToList();
            Assert.NotEmpty(before);

            Stage.WriteWithoutObjects(project, map, TextWriter.Null);
            Archive.Pack(project.StagedFolder, project.ArchivePath);

            var after = Hashes(project.MapFolder).Concat(Hashes(project.AssetsFolder)).ToList();
            Assert.Equal(before, after);
        }

        [Fact]
        public void staged_files()
        {
            var (project, map) = CompiledSample.Value;

            Stage.WriteWithoutObjects(project, map, TextWriter.Null);

            var staged = RelativeFiles(project.StagedFolder);
            foreach (var dropped in DroppedFiles)
            {
                Assert.DoesNotContain(dropped, staged);
            }

            Assert.Equal(map.Script, File.ReadAllText(Path.Combine(project.StagedFolder, "war3map.lua")));
            foreach (var trigger in new[] { "war3map.wtg", "war3map.wct" })
            {
                Assert.Contains(trigger, staged);
                Assert.Equal(
                    File.ReadAllBytes(Path.Combine(project.MapFolder, trigger)),
                    File.ReadAllBytes(Path.Combine(project.StagedFolder, trigger)));
            }

            foreach (var file in RelativeFiles(project.MapFolder))
            {
                if (DroppedFiles.Contains(file) || file == "war3map.lua" || file == "war3map.w3i")
                {
                    continue;
                }

                Assert.Equal(
                    File.ReadAllBytes(Path.Combine(project.MapFolder, file)),
                    File.ReadAllBytes(Path.Combine(project.StagedFolder, file)));
            }

            Assert.Contains(@"Moonwell\hello.txt", staged);
            Assert.Equal(
                File.ReadAllBytes(Path.Combine(project.AssetsFolder, "Moonwell", "hello.txt")),
                File.ReadAllBytes(Path.Combine(project.StagedFolder, "Moonwell", "hello.txt")));
        }

        [Fact]
        public void info_has_settings()
        {
            var (project, map) = CompiledSample.Value;

            Stage.WriteWithoutObjects(project, map, TextWriter.Null);

            using var reader = new BinaryReader(File.OpenRead(Path.Combine(project.StagedFolder, "war3map.w3i")));
            var info = reader.ReadMapInfo();
            Assert.Equal("Sample map", info.MapName);
            Assert.Equal("Moonwell", info.MapAuthor);
            Assert.Equal("Built by the tests.", info.MapDescription);
            Assert.Equal("Any", info.RecommendedPlayers);
            Assert.Equal(ScriptLanguage.Lua, info.ScriptLanguage);
        }

        [Fact]
        public void info_round_trip()
        {
            var project = MapProject.Find(folders.TemporaryCopyOfSample());
            var settings = new MapSettings("Sample.SampleMap", string.Empty, string.Empty, string.Empty, string.Empty);
            var map = SourceMap.Open(project, settings, TextWriter.Null);
            map.Script = "-- script";

            Stage.WriteWithoutObjects(project, map, TextWriter.Null);

            Assert.Equal(
                File.ReadAllBytes(Path.Combine(project.MapFolder, "war3map.w3i")),
                File.ReadAllBytes(Path.Combine(project.StagedFolder, "war3map.w3i")));
        }

        [Fact]
        public void asset_wins_over_map_file()
        {
            var project = MapProject.Find(folders.TemporaryCopyOfSample());
            var map = OpenWithPlaceholderScript(project);
            File.WriteAllText(Path.Combine(project.AssetsFolder, "war3map.wts"), "STRING 1 { replaced }");
            var log = new StringWriter();

            Stage.WriteWithoutObjects(project, map, log);

            Assert.Equal("STRING 1 { replaced }", File.ReadAllText(Path.Combine(project.StagedFolder, "war3map.wts")));
            Assert.Contains(@"note: assets\war3map.wts replaces the map's file", log.ToString());
        }

        [Theory]
        [InlineData("assets", @"Ljud\Hälsning.txt", "ä")]
        [InlineData("assets", "Pris€.txt", "€")]
        [InlineData("map", @"war3mapImported\Öga.mdx", "Ö")]
        public void a_file_named_past_ascii_is_refused_naming_it(string folder, string file, string letter)
        {
            // War3Net hashes a name's UTF-16 letters, the game its bytes; from U+0200 War3Net cannot pack the name at all.
            var project = MapProject.Find(folders.TemporaryCopyOfSample());
            var map = OpenWithPlaceholderScript(project);
            var path = Path.Combine(folder == "map" ? project.MapFolder : project.AssetsFolder, file);
            Directory.CreateDirectory(Path.GetDirectoryName(path)!);
            File.WriteAllText(path, "x");

            var error = Assert.Throws<BuildException>(() => Stage.WriteWithoutObjects(project, map, TextWriter.Null));

            Assert.Equal($@"{folder}\{file} has ""{letter}"" in its name; a file in the map must be named with ASCII characters only, so rename it", error.Message);
        }

        [Fact]
        public void object_files_are_written_by_object_files()
        {
            var project = MapProject.Find(folders.TemporaryCopyOfSample());
            var map = OpenWithPlaceholderScript(project);
            var fixture = TestFolders.Fixture("WorldEditorObjects");
            foreach (var name in new[] { "war3map.w3u", "war3mapSkin.w3u" })
            {
                File.Copy(Path.Combine(fixture, name), Path.Combine(project.MapFolder, name));
            }

            // A map that does not use Moonwell.Objects has its object files come through unchanged, with no word about them.
            var log = new StringWriter();
            Stage.WriteWithoutObjects(project, map, log);

            foreach (var name in new[] { "war3map.w3u", "war3mapSkin.w3u" })
            {
                Assert.Equal(File.ReadAllBytes(Path.Combine(fixture, name)), File.ReadAllBytes(Path.Combine(project.StagedFolder, name)));
            }

            Assert.Equal(string.Empty, log.ToString());

            // A declared object is written beside them.
            var declared = new DeclaredObject(
                "Objects.Peasant",
                "unit",
                "hpea",
                false,
                null,
                null,
                new SortedDictionary<string, object?>(),
                Array.Empty<string>());
            Stage.Write(
                project,
                map,
                new[] { declared },
                new Dictionary<string, string> { ["Objects.Peasant"] = "h001" },
                new Dictionary<string, string>(),
                StandardData.Empty,
                TextWriter.Null);

            Assert.NotEqual(File.ReadAllBytes(Path.Combine(fixture, "war3map.w3u")), File.ReadAllBytes(Path.Combine(project.StagedFolder, "war3map.w3u")));
        }

        [Fact]
        public void stale_staged_files_are_gone()
        {
            var project = MapProject.Find(folders.TemporaryCopyOfSample());
            var map = OpenWithPlaceholderScript(project);
            Stage.WriteWithoutObjects(project, map, TextWriter.Null);
            Archive.Pack(project.StagedFolder, project.ArchivePath);
            Assert.True(File.Exists(Path.Combine(project.StagedFolder, "Moonwell", "hello.txt")));

            File.Delete(Path.Combine(project.AssetsFolder, "Moonwell", "hello.txt"));
            Stage.WriteWithoutObjects(project, map, TextWriter.Null);
            Archive.Pack(project.StagedFolder, project.ArchivePath);

            Assert.False(File.Exists(Path.Combine(project.StagedFolder, "Moonwell", "hello.txt")));
            using var archive = MpqArchive.Open(project.ArchivePath, true);
            Assert.False(archive.FileExists(@"Moonwell\hello.txt"));
            Assert.True(archive.FileExists("war3map.lua"));
        }

        [Fact]
        public void archive_matches_stage()
        {
            var project = MapProject.Find(folders.TemporaryCopyOfSample());
            var map = OpenWithPlaceholderScript(project);
            Stage.WriteWithoutObjects(project, map, TextWriter.Null);

            Archive.Pack(project.StagedFolder, project.ArchivePath);

            var staged = RelativeFiles(project.StagedFolder);
            using (var archive = MpqArchive.Open(project.ArchivePath, true))
            {
                foreach (var file in staged)
                {
                    Assert.True(archive.FileExists(file), file);
                    using var stream = archive.OpenFile(file);
                    using var copy = new MemoryStream();
                    stream.CopyTo(copy);
                    Assert.Equal(File.ReadAllBytes(Path.Combine(project.StagedFolder, file)), copy.ToArray());
                }

                Assert.True(archive.FileExists("(listfile)"));
                using var names = archive.OpenFile("(listfile)");
                using var reader = new StreamReader(names);
                var listed = reader.ReadToEnd()
                    .Split(new[] { '\r', '\n' }, StringSplitOptions.RemoveEmptyEntries)
                    .OrderBy(name => name, StringComparer.OrdinalIgnoreCase)
                    .ToList();
                Assert.Equal(staged.OrderBy(name => name, StringComparer.OrdinalIgnoreCase).ToList(), listed);
                Assert.Equal(staged.Count + 1, archive.GetMpqFiles().Count());
            }

            // Packing leaves no staged file open.
            Directory.Delete(project.StagedFolder, true);
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        private static War3Net.Build.Map OpenWithPlaceholderScript(MapProject project)
        {
            var settings = new MapSettings("Sample.SampleMap", "Sample map", "Moonwell", "Built by the tests.", "Any");
            var map = SourceMap.Open(project, settings, TextWriter.Null);
            map.Script = "-- script";
            return map;
        }

        private static List<string> RelativeFiles(string folder)
        {
            return Directory.GetFiles(folder, "*", SearchOption.AllDirectories)
                .Select(file => Path.GetRelativePath(folder, file))
                .OrderBy(file => file, StringComparer.Ordinal)
                .ToList();
        }

        private static List<string> Hashes(string folder)
        {
            return RelativeFiles(folder)
                .Select(file => file + " " + Convert.ToHexString(SHA256.HashData(File.ReadAllBytes(Path.Combine(folder, file)))))
                .ToList();
        }
    }
}
