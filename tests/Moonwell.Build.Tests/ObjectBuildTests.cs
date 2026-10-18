using System.Reflection;
using System.Text;
using System.Text.RegularExpressions;
using War3Net.Build.Extensions;
using War3Net.Build.Info;
using Xunit;

namespace Moonwell.Build.Tests
{
    [Collection("Sample")]
    public sealed class ObjectBuildTests : IDisposable
    {
        private const string RecommendedPlayers = "RecommendedPlayers = \"Any\";";

        private readonly SampleBuilds builds;
        private readonly TestFolders folders = new TestFolders();

        public ObjectBuildTests(SampleBuilds builds)
        {
            this.builds = builds;
        }

        public void Dispose()
        {
            folders.Dispose();
        }

        [Fact]
        public void sample_builds_with_objects()
        {
            foreach (var built in new[] { builds.Development, builds.Release })
            {
                Assert.Equal(0, built.ExitCode);
                Assert.Equal(string.Empty, built.Error);
                Assert.DoesNotContain("warning", built.Output);
            }

            var sample = TestFolders.Fixture("Sample");
            var ids = ReadIds(Path.Combine(sample, "ids.json"));
            var entry = Assert.Single(ids);
            Assert.Equal("Sample.Arthas", entry.Key);

            var staged = Path.Combine(builds.Development.BuildFolder, "Sample");
            using (var reader = new BinaryReader(File.OpenRead(Path.Combine(staged, "war3map.w3u")), Encoding.UTF8))
            {
                var arthas = Assert.Single(reader.ReadUnitObjectData().NewUnits);
                Assert.Equal(IdOf(entry.Value), arthas.NewId);
                Assert.Equal(IdOf("Hpal"), arthas.OldId);
            }

            using (var reader = new BinaryReader(File.OpenRead(Path.Combine(staged, "war3mapSkin.w3u")), Encoding.UTF8))
            {
                var name = Assert.Single(Assert.Single(reader.ReadUnitObjectData().NewUnits).Modifications, one => one.Id == IdOf("unam"));
                Assert.Equal("Arthas", name.Value);
            }

            foreach (var script in new[] { builds.Development.Script, builds.Release.Script })
            {
                Assert.Contains("namespace.class(\"Paladin\"", script);
                Assert.Contains("MoonwellObjectValues", script);
                Assert.Contains("[\"Sample.Arthas\"]", script);
                Assert.DoesNotContain("Archmage", script);
            }

            // Read from the build's own copy: other tests build the Sample in place.
            var generated = Path.Combine(builds.Development.BuildFolder, "generated");
            Assert.Contains("ObjectRegistry.Register", File.ReadAllText(Path.Combine(generated, "MapMain.g.cs")));
            Assert.Contains($"Id<global::Sample.Arthas>(global::Moonwell.ObjectTypes.FourCC(\"{entry.Value}\"))", File.ReadAllText(Path.Combine(generated, "ObjectRegistry.g.cs")));
        }

        [Fact]
        public void ids_unchanged_on_rebuild()
        {
            var committed = File.ReadAllBytes(Path.Combine(TestFolders.Fixture("Sample"), "ids.json"));
            var copy = folders.TemporaryCopyOfSample();
            Assert.Equal(committed, File.ReadAllBytes(Path.Combine(copy, "ids.json")));

            MapBuild.Run(copy, new BuildOptions(), TextWriter.Null);
            Assert.Equal(committed, File.ReadAllBytes(Path.Combine(copy, "ids.json")));

            MapBuild.Run(copy, new BuildOptions(), TextWriter.Null);
            Assert.Equal(committed, File.ReadAllBytes(Path.Combine(copy, "ids.json")));
        }

        [Theory]
        [InlineData(0, GameDataSet.Default, GameDataVersion.Unset, null)]
        [InlineData(1, GameDataSet.Melee, GameDataVersion.RoC, "melee_v0")]
        [InlineData(2, GameDataSet.Custom, GameDataVersion.RoC, "custom_v0")]
        [InlineData(3, GameDataSet.Custom, GameDataVersion.TFT, "custom_v1")]
        public void balance_writes_option(int balance, GameDataSet set, GameDataVersion version, string? tables)
        {
            var project = MapProject.Find(folders.TemporaryCopyOfSample());
            var settings = new MapSettings("Sample.SampleMap", string.Empty, string.Empty, string.Empty, string.Empty, balance);
            var map = SourceMap.Open(project, settings);
            map.Script = "-- script";

            Stage.WriteWithoutObjects(project, map, TextWriter.Null);

            using var reader = new BinaryReader(File.OpenRead(Path.Combine(project.StagedFolder, "war3map.w3i")));
            var info = reader.ReadMapInfo();
            Assert.Equal(set, info.GameDataSet);
            Assert.Equal(version, info.GameDataVersion);
            Assert.Equal(tables, GameBalance.SetOf(balance));
        }

        [Fact]
        public void unknown_balance_is_an_error()
        {
            var error = Assert.Throws<BuildException>(() => GameBalance.SetOf(9));

            Assert.Equal("Balance 9 is not a balance of this version of Moonwell", error.Message);
        }

        [Fact]
        public void balance_numbers_match_the_runtime()
        {
            // Read from the Sample's own copy of Moonwell, from bytes so that no file stays locked.
            Assert.Equal(0, builds.Development.ExitCode);
            var dll = Path.Combine(TestFolders.Fixture("Sample"), "bin", "Debug", "net10.0", "Moonwell.dll");
            var balance = Assembly.Load(File.ReadAllBytes(dll)).GetType("Moonwell.Balance", true)!;

            var expected = new Dictionary<string, string?>
            {
                ["Default"] = null,
                ["MeleeV0"] = "melee_v0",
                ["CustomV0"] = "custom_v0",
                ["CustomV1"] = "custom_v1",
            };
            Assert.Equal(expected.Keys.Order(StringComparer.Ordinal), Enum.GetNames(balance).Order(StringComparer.Ordinal));
            foreach (var name in expected.Keys)
            {
                Assert.Equal(expected[name], GameBalance.SetOf((int)Enum.Parse(balance, name)));
            }
        }

        [Fact]
        public void balance_of_the_map_class_reaches_the_info_file()
        {
            var copy = folders.TemporaryCopyOfSample();
            SetBalance(copy, "CustomV1");

            MapBuild.Run(copy, new BuildOptions(), TextWriter.Null);

            using var reader = new BinaryReader(File.OpenRead(Path.Combine(copy, "build", "Sample", "war3map.w3i")));
            var info = reader.ReadMapInfo();
            Assert.Equal(GameDataSet.Custom, info.GameDataSet);
            Assert.Equal(GameDataVersion.TFT, info.GameDataVersion);
        }

        [Fact]
        public void declared_object_missing_from_the_balance_set_is_an_error()
        {
            var copy = folders.TemporaryCopyOfSample();
            SetBalance(copy, "CustomV0");
            File.WriteAllText(
                Path.Combine(copy, "Illidan.cs"),
                "namespace Sample\n{\n    public sealed class Illidan : Moonwell.Objects.Units.Campaign.IllidanEevi\n    {\n    }\n}\n");

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), TextWriter.Null));

            Assert.Equal("Sample.Illidan: Eevi does not exist in custom_v0; its balance set leaves it out", error.Message);
        }

        [Fact]
        public void used_standard_class_missing_from_the_balance_set_is_an_error()
        {
            var copy = folders.TemporaryCopyOfSample();
            SetBalance(copy, "MeleeV0");
            File.WriteAllText(
                Path.Combine(copy, "Spawn.cs"),
                "using Moonwell;\n\nnamespace Sample\n{\n    public sealed class Spawn : GameComponent\n    {\n"
                + "        public override void OnUnitDied(Unit dying, Unit? killer)\n        {\n"
                + "            Unit.Create<Moonwell.Objects.Units.Campaign.IllidanEevi>(Player.Get(0), 0f, 0f, 0f);\n        }\n    }\n}\n");
            ReplaceInFile(Path.Combine(copy, "Map.cs"), "Components.Add(new Hello());", "Components.Add(new Hello());\n            Components.Add(new Spawn());");

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), TextWriter.Null));

            Assert.Equal("Moonwell.Objects.Units.Campaign.IllidanEevi: Eevi does not exist in melee_v0; its balance set leaves it out", error.Message);
        }

        [Fact]
        public void object_in_every_balance_set_builds_under_melee()
        {
            var copy = folders.TemporaryCopyOfSample();
            SetBalance(copy, "MeleeV0");

            var archive = MapBuild.Run(copy, new BuildOptions(), TextWriter.Null);

            Assert.True(File.Exists(archive));
            using var reader = new BinaryReader(File.OpenRead(Path.Combine(copy, "build", "Sample", "war3map.w3i")));
            Assert.Equal(GameDataSet.Melee, reader.ReadMapInfo().GameDataSet);
        }

        [Fact]
        public void project_without_objects_builds_with_its_object_files_as_they_are()
        {
            var copy = folders.TemporaryCopyOfSample();
            File.Delete(Path.Combine(copy, "Objects.cs"));
            File.Delete(Path.Combine(copy, "ids.json"));
            var project = Path.Combine(copy, "Sample.csproj");
            var text = File.ReadAllText(project);
            var without = Regex.Replace(text, @"[ \t]*<ProjectReference [^>]*Moonwell\.Objects\.csproj"" />\r?\n", string.Empty);
            Assert.NotEqual(text, without);
            File.WriteAllText(project, without);
            var fixture = TestFolders.Fixture("WorldEditorObjects");
            foreach (var name in new[] { "war3map.w3u", "war3mapSkin.w3u" })
            {
                File.Copy(Path.Combine(fixture, name), Path.Combine(copy, "map", name), true);
            }

            // A registry an earlier build left would not compile without Moonwell.Objects.
            Directory.CreateDirectory(Path.Combine(copy, "build", "generated"));
            File.WriteAllText(Path.Combine(copy, "build", "generated", "ObjectRegistry.g.cs"), "namespace Left { public class Over : DoesNotExist { } }\n");

            var log = new StringWriter();
            var archive = MapBuild.Run(copy, new BuildOptions(), log);

            Assert.True(File.Exists(archive), log.ToString());
            Assert.DoesNotContain("warning", log.ToString());
            Assert.False(File.Exists(Path.Combine(copy, "ids.json")));
            Assert.False(File.Exists(Path.Combine(copy, "build", "generated", "ObjectRegistry.g.cs")));
            foreach (var name in new[] { "war3map.w3u", "war3mapSkin.w3u" })
            {
                Assert.Equal(File.ReadAllBytes(Path.Combine(fixture, name)), File.ReadAllBytes(Path.Combine(copy, "build", "Sample", name)));
            }

            Assert.DoesNotContain("MoonwellObjectValues = {", File.ReadAllText(Path.Combine(copy, "build", "Sample", "war3map.lua")));
        }

        [Fact]
        public void a_stale_objects_library_beside_the_assembly_does_not_make_the_objects_build()
        {
            var copy = folders.TemporaryCopyOfSample();
            File.Delete(Path.Combine(copy, "Objects.cs"));
            File.Delete(Path.Combine(copy, "ids.json"));
            var project = Path.Combine(copy, "Sample.csproj");
            var text = File.ReadAllText(project);
            File.WriteAllText(project, Regex.Replace(text, @"[ 	]*<ProjectReference [^>]*Moonwell\.Objects\.csproj"" />?
", string.Empty));

            // The library an earlier build of the project, when it still used Moonwell.Objects, left in its output.
            var output = Path.Combine(copy, "bin", "Debug", "net10.0");
            Directory.CreateDirectory(output);
            File.Copy(Path.Combine(TestFolders.Fixture("Sample"), "bin", "Debug", "net10.0", "Moonwell.Objects.dll"), Path.Combine(output, "Moonwell.Objects.dll"));

            var log = new StringWriter();
            var archive = MapBuild.Run(copy, new BuildOptions(), log);

            Assert.True(File.Exists(archive), log.ToString());
            Assert.True(File.Exists(Path.Combine(output, "Moonwell.Objects.dll")));
            Assert.False(File.Exists(Path.Combine(copy, "ids.json")));
            Assert.DoesNotContain("MoonwellObjectValues = {", File.ReadAllText(Path.Combine(copy, "build", "Sample", "war3map.lua")));
        }

        [Fact]
        public void source_map_ids_are_those_of_its_object_files()
        {
            var copy = folders.TemporaryCopyOfSample();
            var project = MapProject.Find(copy);
            Assert.Empty(MapObjectIds.Defined(project));
            var fixture = TestFolders.Fixture("WorldEditorObjects");
            foreach (var name in new[] { "war3map.w3u", "war3mapSkin.w3u" })
            {
                File.Copy(Path.Combine(fixture, name), Path.Combine(project.MapFolder, name), true);
            }

            var ids = MapObjectIds.Defined(project);

            Assert.Contains("h000", ids);
        }

        [Fact]
        public void placed_ids_are_those_of_the_units_and_doodads_the_map_places()
        {
            var project = MapProject.Find(folders.TemporaryCopyOfSample());
            var map = SourceMap.Open(project, new MapSettings("Sample.SampleMap", string.Empty, string.Empty, string.Empty, string.Empty));

            var placed = MapObjectIds.Placed(map);

            Assert.NotEmpty(placed);
            foreach (var unit in map.Units!.Units)
            {
                Assert.Contains(Encoding.ASCII.GetString(BitConverter.GetBytes(unit.TypeId)), placed);
            }

            foreach (var doodad in map.Doodads!.Doodads)
            {
                Assert.Contains(Encoding.ASCII.GetString(BitConverter.GetBytes(doodad.TypeId)), placed);
            }
        }

        private static void SetBalance(string folder, string balance)
        {
            ReplaceInFile(Path.Combine(folder, "Map.cs"), RecommendedPlayers, RecommendedPlayers + "\n            Balance = Balance." + balance + ";");
        }

        private static void ReplaceInFile(string path, string text, string with)
        {
            var before = File.ReadAllText(path);
            var after = before.Replace(text, with, StringComparison.Ordinal);
            Assert.NotEqual(before, after);
            File.WriteAllText(path, after);
        }

        private static Dictionary<string, string> ReadIds(string path)
        {
            var ids = new Dictionary<string, string>();
            foreach (Match match in Regex.Matches(File.ReadAllText(path), "\"([^\"]+)\": \"([^\"]+)\""))
            {
                ids[match.Groups[1].Value] = match.Groups[2].Value;
            }

            return ids;
        }

        private static int IdOf(string text)
        {
            return BitConverter.ToInt32(Encoding.ASCII.GetBytes(text), 0);
        }
    }
}
