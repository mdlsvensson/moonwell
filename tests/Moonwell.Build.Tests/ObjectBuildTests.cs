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

        // DataSet: 0 Default, 1 Custom, 2 Melee. DataVersion: 0 ReignOfChaos, 1 FrozenThrone, 2 ForsakenKingdom.
        [Theory]
        [InlineData(0, 2, GameDataSet.Default, GameDataVersion.Unset, null, null)]
        [InlineData(1, 2, GameDataSet.Custom, GameDataVersion.Unset, null, null)]
        [InlineData(2, 2, GameDataSet.Melee, GameDataVersion.Unset, null, null)]
        [InlineData(0, 1, GameDataSet.Default, GameDataVersion.TFT, "custom_v1", "custom_v1")]
        [InlineData(1, 1, GameDataSet.Custom, GameDataVersion.TFT, "custom_v1", "custom_v1")]
        [InlineData(2, 1, GameDataSet.Melee, GameDataVersion.TFT, null, "custom_v1")]
        [InlineData(0, 0, GameDataSet.Default, GameDataVersion.RoC, "custom_v0", "custom_v0")]
        [InlineData(1, 0, GameDataSet.Custom, GameDataVersion.RoC, "custom_v0", "custom_v0")]
        [InlineData(2, 0, GameDataSet.Melee, GameDataVersion.RoC, "melee_v0", "melee_v0")]
        public void data_option_writes_info_values_and_reads_tables(int dataSet, int dataVersion, GameDataSet set, GameDataVersion version, string? values, string? existence)
        {
            var project = MapProject.Find(folders.TemporaryCopyOfSample());
            var settings = new MapSettings("Sample.SampleMap", string.Empty, string.Empty, string.Empty, string.Empty, dataSet, dataVersion);
            var map = SourceMap.Open(project, settings, TextWriter.Null);
            map.Script = "-- script";

            Stage.WriteWithoutObjects(project, map, TextWriter.Null);

            using var reader = new BinaryReader(File.OpenRead(Path.Combine(project.StagedFolder, "war3map.w3i")));
            var info = reader.ReadMapInfo();
            Assert.Equal(set, info.GameDataSet);
            Assert.Equal(version, info.GameDataVersion);
            var data = GameData.Of(dataSet, dataVersion);
            Assert.Equal(values, data.ValueSet);
            Assert.Equal(existence, data.ExistenceSet);
        }

        [Theory]
        [InlineData(0, 2, null)]
        [InlineData(2, 0, "note: the map's game data option Default / Forsaken Kingdom is replaced by Melee / Reign of Chaos")]
        [InlineData(1, 0, "note: the map's game data option Default / Forsaken Kingdom is replaced by Custom / Reign of Chaos")]
        [InlineData(2, 1, "note: the map's game data option Default / Forsaken Kingdom is replaced by Melee / Frozen Throne")]
        [InlineData(0, 1, "note: the map's game data option Default / Forsaken Kingdom is replaced by Default / Frozen Throne")]
        [InlineData(1, 2, "note: the map's game data option Default / Forsaken Kingdom is replaced by Custom / Forsaken Kingdom")]
        public void data_option_that_changes_the_game_data_option_is_noted(int dataSet, int dataVersion, string? note)
        {
            // The Sample's World Editor map has the default option.
            var project = MapProject.Find(folders.TemporaryCopyOfSample());
            var log = new StringWriter { NewLine = "\n" };

            SourceMap.Open(project, new MapSettings("Sample.SampleMap", string.Empty, string.Empty, string.Empty, string.Empty, dataSet, dataVersion), log);

            Assert.Equal(note == null ? string.Empty : note + "\n", log.ToString());
        }

        [Fact]
        public void a_changed_option_is_noted_with_the_one_the_map_had()
        {
            var copy = folders.TemporaryCopyOfSample();
            var infoPath = Path.Combine(copy, "map", "war3map.w3i");
            MapInfo info;
            using (var reader = new BinaryReader(File.OpenRead(infoPath)))
            {
                info = reader.ReadMapInfo();
            }

            info.GameDataSet = GameDataSet.Custom;
            info.GameDataVersion = GameDataVersion.TFT;
            using (var writer = new BinaryWriter(File.Create(infoPath)))
            {
                writer.Write(info);
            }

            var project = MapProject.Find(copy);
            var log = new StringWriter { NewLine = "\n" };

            SourceMap.Open(project, new MapSettings("Sample.SampleMap", string.Empty, string.Empty, string.Empty, string.Empty, 2, 0), log);
            Assert.Equal("note: the map's game data option Custom / Frozen Throne is replaced by Melee / Reign of Chaos\n", log.ToString());

            log.GetStringBuilder().Clear();
            SourceMap.Open(project, new MapSettings("Sample.SampleMap", string.Empty, string.Empty, string.Empty, string.Empty, 1, 1), log);
            Assert.Equal(string.Empty, log.ToString());
        }

        [Fact]
        public void unknown_data_set_or_version_is_an_error()
        {
            Assert.Equal("DataSet 9 is not a data set of this version of Moonwell", Assert.Throws<BuildException>(() => GameData.Of(9, 0)).Message);
            Assert.Equal("DataVersion 9 is not a data version of this version of Moonwell", Assert.Throws<BuildException>(() => GameData.Of(0, 9)).Message);
        }

        [Fact]
        public void data_set_and_version_numbers_match_the_runtime()
        {
            // Read from the Sample's own copy of Moonwell, from bytes so that no file stays locked.
            Assert.Equal(0, builds.Development.ExitCode);
            var dll = Path.Combine(TestFolders.Fixture("Sample"), "bin", "Debug", "net10.0", "Moonwell.dll");
            var runtime = Assembly.Load(File.ReadAllBytes(dll));
            var dataSet = runtime.GetType("Moonwell.DataSet", true)!;
            var dataVersion = runtime.GetType("Moonwell.DataVersion", true)!;

            var sets = new Dictionary<string, GameDataSet>
            {
                ["Default"] = GameDataSet.Default,
                ["Custom"] = GameDataSet.Custom,
                ["Melee"] = GameDataSet.Melee,
            };
            var versions = new Dictionary<string, GameDataVersion>
            {
                ["ReignOfChaos"] = GameDataVersion.RoC,
                ["FrozenThrone"] = GameDataVersion.TFT,
                ["ForsakenKingdom"] = GameDataVersion.Unset,
            };
            Assert.Equal(sets.Keys.Order(StringComparer.Ordinal), Enum.GetNames(dataSet).Order(StringComparer.Ordinal));
            Assert.Equal(versions.Keys.Order(StringComparer.Ordinal), Enum.GetNames(dataVersion).Order(StringComparer.Ordinal));
            var forsaken = (int)Enum.Parse(dataVersion, "ForsakenKingdom");
            var defaultSet = (int)Enum.Parse(dataSet, "Default");
            foreach (var name in sets.Keys)
            {
                Assert.Equal(sets[name], GameData.Of((int)Enum.Parse(dataSet, name), forsaken).InfoSet);
            }

            foreach (var name in versions.Keys)
            {
                Assert.Equal(versions[name], GameData.Of(defaultSet, (int)Enum.Parse(dataVersion, name)).InfoVersion);
            }
        }

        [Fact]
        public void data_option_of_the_map_class_reaches_the_info_file()
        {
            var copy = folders.TemporaryCopyOfSample();
            SetData(copy, "Custom", "FrozenThrone");

            MapBuild.Run(copy, new BuildOptions(), TextWriter.Null);

            using var reader = new BinaryReader(File.OpenRead(Path.Combine(copy, "build", "Sample", "war3map.w3i")));
            var info = reader.ReadMapInfo();
            Assert.Equal(GameDataSet.Custom, info.GameDataSet);
            Assert.Equal(GameDataVersion.TFT, info.GameDataVersion);
        }

        [Fact]
        public void declared_object_missing_from_the_game_data_option_is_an_error()
        {
            var copy = folders.TemporaryCopyOfSample();
            SetData(copy, "Custom", "ReignOfChaos");
            File.WriteAllText(
                Path.Combine(copy, "Illidan.cs"),
                "namespace Sample\n{\n    public sealed class Illidan : Moonwell.Objects.Campaign.Units.IllidanEevi\n    {\n    }\n}\n");

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), TextWriter.Null));

            Assert.Equal("Sample.Illidan: Eevi does not exist under Custom / Reign of Chaos", error.Message);
        }

        [Fact]
        public void used_standard_class_missing_from_the_game_data_option_is_an_error()
        {
            var copy = folders.TemporaryCopyOfSample();
            SetData(copy, "Melee", "ReignOfChaos");
            UseIllidan(copy);

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), TextWriter.Null));

            Assert.Equal("Moonwell.Objects.Campaign.Units.IllidanEevi: Eevi does not exist under Melee / Reign of Chaos", error.Message);
        }

        [Fact]
        public void melee_with_frozen_throne_has_the_objects_of_custom_v1()
        {
            // Eevi (Illidan) is in custom_v1 but not in melee_v0, so it exists here; the spur is in neither custom_v1 nor the base.
            var copy = folders.TemporaryCopyOfSample();
            SetData(copy, "Melee", "FrozenThrone");
            UseIllidan(copy);
            Assert.True(File.Exists(MapBuild.Run(copy, new BuildOptions(), TextWriter.Null)));

            File.WriteAllText(
                Path.Combine(copy, "Spurred.cs"),
                "namespace Sample\n{\n    public sealed class Spurred : Moonwell.Objects.Items.spur\n    {\n    }\n}\n");

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), TextWriter.Null));

            Assert.Equal("Sample.Spurred: spur does not exist under Melee / Frozen Throne", error.Message);
        }

        [Fact]
        public void melee_with_frozen_throne_leaves_out_what_only_the_base_tables_have()
        {
            // The Forsaken Apothecary (fapo) is in the base tables only: Melee / Forsaken Kingdom reads them, Melee / Frozen Throne reads custom_v1.
            var copy = folders.TemporaryCopyOfSample();
            File.WriteAllText(
                Path.Combine(copy, "Poisoner.cs"),
                "namespace Sample\n{\n    public sealed class Poisoner : Moonwell.Objects.Campaign.Units.Apothecary\n    {\n    }\n}\n");
            SetData(copy, "Melee", "ForsakenKingdom");
            Assert.True(File.Exists(MapBuild.Run(copy, new BuildOptions(), TextWriter.Null)));

            ReplaceInFile(Path.Combine(copy, "Map.cs"), "DataVersion = DataVersion.ForsakenKingdom;", "DataVersion = DataVersion.FrozenThrone;");

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), TextWriter.Null));

            Assert.Equal("Sample.Poisoner: fapo does not exist under Melee / Frozen Throne", error.Message);
        }

        [Fact]
        public void object_in_every_set_of_tables_builds_under_melee()
        {
            foreach (var version in new[] { "ReignOfChaos", "FrozenThrone", "ForsakenKingdom" })
            {
                var copy = folders.TemporaryCopyOfSample();
                SetData(copy, "Melee", version);

                var archive = MapBuild.Run(copy, new BuildOptions(), TextWriter.Null);

                Assert.True(File.Exists(archive));
                using var reader = new BinaryReader(File.OpenRead(Path.Combine(copy, "build", "Sample", "war3map.w3i")));
                Assert.Equal(GameDataSet.Melee, reader.ReadMapInfo().GameDataSet);
            }
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

            // A registry or stubs an earlier build left would not compile without Moonwell.Objects.
            Directory.CreateDirectory(Path.Combine(copy, "build", "generated"));
            File.WriteAllText(Path.Combine(copy, "build", "generated", "ObjectRegistry.g.cs"), "namespace Left { public class Over : DoesNotExist { } }\n");
            File.WriteAllText(Path.Combine(copy, "build", "generated", "ObjectStubs.g.cs"), "namespace Left { public class Stub : DoesNotExist { } }\n");

            var log = new StringWriter();
            var archive = MapBuild.Run(copy, new BuildOptions(), log);

            Assert.True(File.Exists(archive), log.ToString());
            Assert.DoesNotContain("warning", log.ToString());
            Assert.False(File.Exists(Path.Combine(copy, "ids.json")));
            Assert.False(File.Exists(Path.Combine(copy, "build", "generated", "ObjectRegistry.g.cs")));
            Assert.False(File.Exists(Path.Combine(copy, "build", "generated", "ObjectStubs.g.cs")));
            foreach (var name in new[] { "war3map.w3u", "war3mapSkin.w3u" })
            {
                Assert.Equal(File.ReadAllBytes(Path.Combine(fixture, name)), File.ReadAllBytes(Path.Combine(copy, "build", "Sample", name)));
            }

            Assert.DoesNotContain("MoonwellObjectValues = {", File.ReadAllText(Path.Combine(copy, "build", "Sample", "war3map.lua")));
        }

        [Fact]
        public void a_referenced_name_brings_every_overload_into_the_stubs()
        {
            // The constructor calls only Set(field, value); CSharp.lua names overloads by their order, so the stub keeps both.
            var copy = folders.TemporaryCopyOfSample();
            File.WriteAllText(Path.Combine(copy, "Objects.cs"), """
                namespace Sample
                {
                    public sealed class Arthas : Moonwell.Objects.Human.Units.Paladin
                    {
                        public Arthas()
                        {
                            Set("unam", "Arthas");
                        }
                    }
                }
                """);

            MapBuild.Run(copy, new BuildOptions(), new StringWriter());

            var stubs = File.ReadAllText(Path.Combine(copy, "build", "generated", "ObjectStubs.g.cs"));
            var first = stubs.IndexOf("protected void Set(string field, object value)", StringComparison.Ordinal);
            var second = stubs.IndexOf("protected void Set(string field, int level, object value)", StringComparison.Ordinal);
            Assert.True(first >= 0 && second > first, stubs);
            Assert.DoesNotContain("HitPoints", stubs);
            var script = File.ReadAllText(Path.Combine(copy, "build", "Sample", "war3map.lua"));
            Assert.Contains("Set = nothing", script);
            Assert.Contains("Set1 = nothing", script);
        }

        [Fact]
        public void a_name_the_assembly_does_not_reference_is_an_error_that_says_why()
        {
            // nameof leaves no reference in the assembly, so the stubs lack HitPoints and CSharp.lua cannot compile the map.
            var copy = folders.TemporaryCopyOfSample();
            File.AppendAllText(Path.Combine(copy, "Objects.cs"), """

                namespace Sample
                {
                    public static class Names
                    {
                        public static readonly string Life = nameof(Moonwell.Objects.Human.Units.Paladin.HitPoints);
                    }
                }
                """);

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), new StringWriter()));

            Assert.Contains("HitPoints", error.Message);
            Assert.Contains("only the members of Moonwell.Objects that the map's assembly references", error.Message);
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
            var map = SourceMap.Open(project, new MapSettings("Sample.SampleMap", string.Empty, string.Empty, string.Empty, string.Empty), TextWriter.Null);

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

        private static void SetData(string folder, string dataSet, string dataVersion)
        {
            ReplaceInFile(
                Path.Combine(folder, "Map.cs"),
                RecommendedPlayers,
                RecommendedPlayers + "\n            DataSet = DataSet." + dataSet + ";\n            DataVersion = DataVersion." + dataVersion + ";");
        }

        // The map's code uses Illidan, whom the game has in the base tables and in custom_v1 only.
        private static void UseIllidan(string folder)
        {
            File.WriteAllText(
                Path.Combine(folder, "Spawn.cs"),
                "using Moonwell;\n\nnamespace Sample\n{\n    public sealed class Spawn : GameComponent\n    {\n"
                + "        public override void OnUnitDied(Unit dying, Unit? killer)\n        {\n"
                + "            Unit.Create<Moonwell.Objects.Campaign.Units.IllidanEevi>(Player.Get(0), 0f, 0f, 0f);\n        }\n    }\n}\n");
            ReplaceInFile(Path.Combine(folder, "Map.cs"), "Components.Add(new Hello());", "Components.Add(new Hello());\n            Components.Add(new Spawn());");
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
