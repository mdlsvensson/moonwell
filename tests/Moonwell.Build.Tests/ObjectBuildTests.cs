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

            // Sample.Arthas has [ObjectId("H001")].
            var entry = new KeyValuePair<string, string>("Sample.Arthas", "H001");

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
        public void a_leftover_ids_json_changes_nothing()
        {
            var copy = folders.TemporaryCopyOfSample();
            var leftover = "{\n  \"Sample.Arthas\": \"H009\"\n}\n";
            File.WriteAllText(Path.Combine(copy, "ids.json"), leftover);

            MapBuild.Run(copy, new BuildOptions(), TextWriter.Null);

            Assert.Equal(leftover, File.ReadAllText(Path.Combine(copy, "ids.json")));
            using (var reader = new BinaryReader(File.OpenRead(Path.Combine(copy, "build", "Sample", "war3map.w3u")), Encoding.UTF8))
            {
                Assert.Equal(IdOf("H001"), Assert.Single(reader.ReadUnitObjectData().NewUnits).NewId);
            }
        }

        [Fact]
        public void a_build_writes_no_ids_json()
        {
            var copy = folders.TemporaryCopyOfSample();

            MapBuild.Run(copy, new BuildOptions(), TextWriter.Null);

            Assert.False(File.Exists(Path.Combine(copy, "ids.json")));
        }

        // DataSet: 0 Default, 1 Custom, 2 Melee. DataVersion: 0 ReignOfChaos, 1 FrozenThrone, 2 ForsakenKingdom.
        [Theory]
        [InlineData(0, 2, GameDataSet.Default, GameDataVersion.ForsakenKingdom, null, null)]
        [InlineData(1, 2, GameDataSet.Custom, GameDataVersion.ForsakenKingdom, null, null)]
        [InlineData(2, 2, GameDataSet.Melee, GameDataVersion.ForsakenKingdom, null, null)]
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
            var dll = Path.Combine(builds.Folder, "bin", "Debug", "net10.0", "Moonwell.dll");
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
                ["ForsakenKingdom"] = GameDataVersion.ForsakenKingdom,
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
                "namespace Sample\n{\n    [Moonwell.Objects.ObjectId(\"H00I\")]\n    public sealed class Illidan : Moonwell.Objects.Campaign.Units.IllidanEevi\n    {\n    }\n}\n");

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
                "namespace Sample\n{\n    [Moonwell.Objects.ObjectId(\"I00S\")]\n    public sealed class Spurred : Moonwell.Objects.Items.spur\n    {\n    }\n}\n");

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
                "namespace Sample\n{\n    [Moonwell.Objects.ObjectId(\"h00P\")]\n    public sealed class Poisoner : Moonwell.Objects.Campaign.Units.Apothecary\n    {\n    }\n}\n");
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
        public void a_units_level_set_in_a_map_without_objects_fails_the_build()
        {
            // The build reads no declarations of a map without Moonwell.Objects, but the game has no setter of a unit's level.
            var copy = folders.TemporaryCopyOfSample();
            File.Delete(Path.Combine(copy, "Objects.cs"));
            File.Delete(Path.Combine(copy, "ids.json"));
            var project = Path.Combine(copy, "Sample.csproj");
            File.WriteAllText(project, Regex.Replace(File.ReadAllText(project), @"[ \t]*<ProjectReference [^>]*Moonwell\.Objects\.csproj"" />\r?\n", string.Empty));
            // MW0015 reports the same set while compiling; the pragma leaves it to the build's own check.
            File.WriteAllText(Path.Combine(copy, "Drill.cs"), """
                #pragma warning disable MW0015
                using Moonwell;

                namespace Sample
                {
                    public sealed class Drill : GameComponent
                    {
                        public override void OnUnitDied(Unit dying, Unit? killer)
                        {
                            dying.Level = 2;
                        }
                    }
                }
                """);

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), new StringWriter()));

            Assert.Equal("Sample.Drill.OnUnitDied sets Level; a unit's level is set only in its declared class's constructor", error.Message);
        }

        [Fact]
        public void a_class_nested_in_the_map_and_a_using_static_of_a_group_build()
        {
            var copy = folders.TemporaryCopyOfSample();
            File.WriteAllText(Path.Combine(copy, "Objects.cs"), """
                using static Moonwell.Objects.Human;

                namespace Sample
                {
                    public static class Spells
                    {
                        [Moonwell.Objects.ObjectId("A00F")]
                        public sealed class Fireball : Abilities.StormBolt
                        {
                            public Fireball()
                            {
                                Damage = Levels(10f, 20f, 30f);
                            }
                        }
                    }
                }
                """);

            MapBuild.Run(copy, new BuildOptions(), new StringWriter());

            var script = File.ReadAllText(Path.Combine(copy, "build", "Sample", "war3map.lua"));
            Assert.Contains("\"Sample.Spells.Fireball\"", script);
            Assert.Contains("\"Moonwell.Objects.Human.Abilities.StormBolt\"", script);
        }

        [Fact]
        public void beyond_limits_and_a_choice_compile_to_lua_through_the_stubs()
        {
            // The library's files of the attribute and the enums are left out of CSharp.lua's compile; the stubs carry what the map names.
            var copy = folders.TemporaryCopyOfSample();
            File.WriteAllText(Path.Combine(copy, "Objects.cs"), """
                namespace Sample
                {
                    [Moonwell.Objects.ObjectId("H001")]
                    [Moonwell.Objects.BeyondLimits(nameof(MaxLife))]
                    public sealed class Arthas : Moonwell.Objects.Human.Units.Paladin
                    {
                        public Arthas()
                        {
                            MaxLife = 600000;
                            DeathType = Moonwell.Objects.DeathType.CanRaiseDoesNotDecay;
                        }
                    }
                }
                """);

            MapBuild.Run(copy, new BuildOptions(), new StringWriter());

            var stubs = File.ReadAllText(Path.Combine(copy, "build", "generated", "ObjectStubs.g.cs"));
            Assert.Contains("public sealed class BeyondLimitsAttribute : global::System.Attribute", stubs);
            Assert.Contains("public enum DeathType\n", stubs);
            Assert.DoesNotContain("public enum Targets", stubs);
            var script = File.ReadAllText(Path.Combine(copy, "build", "Sample", "war3map.lua"));
            Assert.Contains("\"Sample.Arthas\"", script);
        }

        [Fact]
        public void a_heros_total_becomes_its_base_in_the_object_file()
        {
            // The Paladin has 22 strength and 17 intelligence: 550 life and 255 mana of his start come from them (25 and 15 a
            // point). Mana is what he starts with, his field plus his intelligence's share (seen on 3.0.1, 10-21).
            var copy = folders.TemporaryCopyOfSample();
            File.WriteAllText(Path.Combine(copy, "Objects.cs"), """
                namespace Sample
                {
                    [Moonwell.Objects.ObjectId("H001")]
                    public sealed class Arthas : Moonwell.Objects.Human.Units.Paladin
                    {
                        public Arthas()
                        {
                            MaxLife = 800;
                            MaxMana = 400;
                            Mana = 300;
                        }
                    }
                }
                """);

            MapBuild.Run(copy, new BuildOptions(), new StringWriter());

            var arthas = Assert.Single(ReadUnits(copy).NewUnits);
            Assert.Equal(250, Assert.Single(arthas.Modifications, one => one.Id == IdOf("uhpm")).Value);
            Assert.Equal(145, Assert.Single(arthas.Modifications, one => one.Id == IdOf("umpm")).Value);
            Assert.Equal(45, Assert.Single(arthas.Modifications, one => one.Id == IdOf("umpi")).Value);
        }

        [Fact]
        public void a_float_on_an_int_field_is_truncated()
        {
            // MaxLife is a float, as the game gives it; the field is a whole number, and the setter drops the fraction.
            var copy = folders.TemporaryCopyOfSample();
            File.WriteAllText(Path.Combine(copy, "Objects.cs"), """
                namespace Sample
                {
                    [Moonwell.Objects.ObjectId("h001")]
                    public sealed class Sturdy : Moonwell.Objects.Human.Units.Footman
                    {
                        public Sturdy()
                        {
                            MaxLife = 800.7f;
                        }
                    }
                }
                """);

            MapBuild.Run(copy, new BuildOptions(), new StringWriter());

            var sturdy = Assert.Single(ReadUnits(copy).NewUnits);
            Assert.Equal(800, Assert.Single(sturdy.Modifications, one => one.Id == IdOf("uhpm")).Value);
        }

        [Theory]
        [InlineData("public const int Magic = (int)Moonwell.Objects.Targets.Air;")]
        [InlineData("public static int Pick(int value) { switch (value) { case (int)Moonwell.Objects.Targets.Air: return 1; default: return 0; } }")]
        [InlineData("public static readonly string Label = nameof(Moonwell.Objects.Targets.Air);")]
        public void an_enum_the_map_names_only_in_a_constant_a_case_or_nameof_compiles_to_lua_through_the_stubs(string member)
        {
            // None of these leave a reference to the enum in the assembly: the build reads the map's source for them.
            var copy = folders.TemporaryCopyOfSample();
            File.AppendAllText(Path.Combine(copy, "Objects.cs"), "\nnamespace Sample\n{\n    public static class Names\n    {\n        " + member + "\n    }\n}\n");

            MapBuild.Run(copy, new BuildOptions(), new StringWriter());

            var stubs = File.ReadAllText(Path.Combine(copy, "build", "generated", "ObjectStubs.g.cs"));
            Assert.Contains("    [global::System.Flags]\n    public enum Targets\n", stubs);
            Assert.DoesNotContain("public enum ArmorType", stubs);
        }

        [Fact]
        public void an_enum_named_only_in_code_for_a_debug_build_is_stubbed_for_the_release_build_too()
        {
            // The stubs are written once for both builds, so they hold what either build's code names.
            var copy = folders.TemporaryCopyOfSample();
            File.AppendAllText(Path.Combine(copy, "Objects.cs"), """

                namespace Sample
                {
                    public static class Names
                    {
                #if DEBUG
                        public const int Magic = (int)Moonwell.Objects.Targets.Air;
                #else
                        public const int Magic = (int)Moonwell.Objects.ArmorType.Metal;
                #endif
                    }
                }
                """);

            MapBuild.Run(copy, new BuildOptions { Release = true }, new StringWriter());

            var stubs = File.ReadAllText(Path.Combine(copy, "build", "generated", "ObjectStubs.g.cs"));
            Assert.Contains("public enum Targets\n", stubs);
            Assert.Contains("public enum ArmorType\n", stubs);
        }

        [Fact]
        public void the_rule_attributes_and_the_field_rule_exception_compile_to_lua_through_the_stubs()
        {
            var copy = folders.TemporaryCopyOfSample();
            File.AppendAllText(Path.Combine(copy, "Objects.cs"), """

                namespace Sample
                {
                    public static class Rules
                    {
                        [Moonwell.Objects.NonNegative]
                        public static int Count { get; set; }

                        [Moonwell.Objects.Limits(1, 500000)]
                        public static int Life { get; set; }

                        [Moonwell.Objects.MaxLength(40)]
                        public static string? Title { get; set; }

                        [Moonwell.Objects.MaxEntries(12)]
                        public static string[]? Names { get; set; }

                        [Moonwell.Objects.Required]
                        public static string? Label { get; set; }

                        public static string? Run()
                        {
                            try
                            {
                                return Title;
                            }
                            catch (Moonwell.Objects.FieldRuleException failure)
                            {
                                return failure.FieldId + failure.Message;
                            }
                        }
                    }
                }
                """);

            MapBuild.Run(copy, new BuildOptions(), new StringWriter());

            var stubs = File.ReadAllText(Path.Combine(copy, "build", "generated", "ObjectStubs.g.cs"));
            foreach (var name in new[] { "NonNegative", "Limits", "MaxLength", "MaxEntries", "Required" })
            {
                Assert.Contains("public sealed class " + name + "Attribute : global::System.Attribute", stubs);
            }

            Assert.Contains("public class FieldRuleException : global::System.Exception", stubs);
            Assert.Contains("public string FieldId", stubs);
            Assert.DoesNotContain("BeyondLimitsAttribute", stubs);
        }

        [Fact]
        public void a_live_member_in_a_declared_class_constructor_is_an_error_saying_what_to_set()
        {
            // Life has no field: a unit starts with full life, and the build runs the constructor with no game.
            var copy = folders.TemporaryCopyOfSample();
            File.WriteAllText(Path.Combine(copy, "Objects.cs"), """
                namespace Sample
                {
                    [Moonwell.Objects.ObjectId("h001")]
                    public sealed class Caster : Moonwell.Objects.Human.Units.Footman
                    {
                        public Caster()
                        {
                            Life = 300;
                        }
                    }
                }
                """);
            var log = new StringWriter();

            Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), log));

            Assert.Contains("error MW0012: 'Life' is the unit's value in the game: a unit starts with full life; set MaxLife", log.ToString());
        }

        [Fact]
        public void a_name_of_the_attack_type_of_moonwell_objects_is_an_error_naming_the_core_enum()
        {
            // AttackType and DefenseType are core Moonwell's since 0.2.1; the compiler's own error says only that the name is gone.
            var copy = folders.TemporaryCopyOfSample();
            File.WriteAllText(Path.Combine(copy, "Objects.cs"), """
                namespace Sample
                {
                    [Moonwell.Objects.ObjectId("H001")]
                    public sealed class Arthas : Moonwell.Objects.Human.Units.Paladin
                    {
                        public Arthas()
                        {
                            AttackType = Moonwell.Objects.AttackType.Magic;
                        }
                    }
                }
                """);
            var log = new StringWriter();

            Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), log));

            Assert.Contains("error MW0011: Moonwell.Objects.AttackType is Moonwell.AttackType since Moonwell 0.2.1", log.ToString());
        }

        [Fact]
        public void a_declaration_of_the_attack_type_of_moonwell_objects_fails_with_the_core_enum_named()
        {
            // A field's type stops the compiler before analyzers run, so the build names the core enum itself.
            var copy = folders.TemporaryCopyOfSample();
            File.AppendAllText(Path.Combine(copy, "Objects.cs"), """

                namespace Sample
                {
                    public static class Kinds
                    {
                        public static Moonwell.Objects.AttackType Attack;
                    }
                }
                """);
            var log = new StringWriter();

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), log));

            Assert.Contains("error CS0234", log.ToString());
            Assert.Equal(
                "dotnet build failed" + Environment.NewLine
                + "Moonwell.Objects.AttackType is Moonwell.AttackType since Moonwell 0.2.1, which the damage package uses too: write Moonwell.AttackType, or AttackType with using Moonwell",
                error.Message);
        }

        [Fact]
        public void an_unqualified_attack_type_in_a_declaration_with_using_moonwell_objects_fails_with_the_core_enum_named()
        {
            // With only using Moonwell.Objects, AttackType binds to nothing (CS0246); a field's type stops the compiler before analyzers run.
            var copy = folders.TemporaryCopyOfSample();
            File.WriteAllText(Path.Combine(copy, "Kinds.cs"), """
                using Moonwell.Objects;

                namespace Sample
                {
                    public static class Kinds
                    {
                        public static AttackType Attack;
                    }
                }
                """);
            var log = new StringWriter();

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), log));

            Assert.Contains("error CS0246", log.ToString());
            Assert.Equal(
                "dotnet build failed" + Environment.NewLine
                + "AttackType is Moonwell.AttackType since Moonwell 0.2.1, which the damage package uses too: write Moonwell.AttackType, or AttackType with using Moonwell",
                error.Message);
        }

        [Fact]
        public void the_build_names_each_moved_enum_of_a_cs0246_once()
        {
            var hints = DotnetBuild.MovedHints(new[]
            {
                @"C:\Map\Kinds.cs(5,23): error CS0246: The type or namespace name 'DefenseType' could not be found (are you missing a using directive or an assembly reference?) [C:\Map\Map.csproj]",
                @"C:\Map\Kinds.cs(6,23): error CS0246: The type or namespace name 'AttackType' could not be found (are you missing a using directive or an assembly reference?) [C:\Map\Map.csproj]",
                @"C:\Map\Kinds.cs(6,23): error CS0246: The type or namespace name 'AttackType' could not be found (are you missing a using directive or an assembly reference?) [C:\Map\Map.csproj]",
                @"C:\Map\Kinds.cs(7,23): error CS0246: The type or namespace name 'AttackTypes' could not be found (are you missing a using directive or an assembly reference?) [C:\Map\Map.csproj]",
                @"C:\Map\Kinds.cs(8,23): error CS0246: The type or namespace name 'Other' could not be found (are you missing a using directive or an assembly reference?) [C:\AttackType\Map.csproj]",
            });

            Assert.Equal(new[] { "DefenseType is Moonwell.DefenseType", "AttackType is Moonwell.AttackType" },
                hints.Select(hint => hint.Substring(0, hint.IndexOf(" since", StringComparison.Ordinal))));
        }

        [Fact]
        public void the_build_names_each_moved_enum_of_a_cs0234_once()
        {
            var hints = DotnetBuild.MovedHints(new[]
            {
                @"C:\Map\Kinds.cs(5,40): error CS0234: The type or namespace name 'DefenseType' does not exist in the namespace 'Moonwell.Objects' (are you missing an assembly reference?) [C:\Map\Map.csproj]",
                @"C:\Map\Kinds.cs(6,40): error CS0234: The type or namespace name 'AttackType' does not exist in the namespace 'Moonwell.Objects' (are you missing an assembly reference?) [C:\Map\Map.csproj]",
                @"C:\Map\Kinds.cs(5,40): error CS0234: The type or namespace name 'DefenseType' does not exist in the namespace 'Moonwell.Objects' (are you missing an assembly reference?) [C:\Map\Map.csproj]",
                @"C:\Map\Kinds.cs(7,40): error CS0234: The type or namespace name 'Targetz' does not exist in the namespace 'Moonwell.Objects' (are you missing an assembly reference?) [C:\Map\Map.csproj]",
                @"C:\Map\Kinds.cs(8,40): error CS0234: The type or namespace name 'AttackType' does not exist in the namespace 'Other' (are you missing an assembly reference?) [C:\Moonwell.Objects\Map.csproj]",
            });

            Assert.Equal(new[] { "Moonwell.Objects.DefenseType is Moonwell.DefenseType", "Moonwell.Objects.AttackType is Moonwell.AttackType" },
                hints.Select(hint => hint.Substring(0, hint.IndexOf(" since", StringComparison.Ordinal))));
        }

        [Fact]
        public void a_referenced_name_brings_every_overload_into_the_stubs()
        {
            // The constructor calls only Set(field, value); CSharp.lua names overloads by their order, so the stub keeps both.
            var copy = folders.TemporaryCopyOfSample();
            File.WriteAllText(Path.Combine(copy, "Objects.cs"), """
                namespace Sample
                {
                    [Moonwell.Objects.ObjectId("H001")]
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
            Assert.DoesNotContain("GoldCost", stubs);
            var script = File.ReadAllText(Path.Combine(copy, "build", "Sample", "war3map.lua"));
            Assert.Contains("Set = nothing", script);
            Assert.Contains("Set1 = nothing", script);
        }

        [Fact]
        public void a_name_the_assembly_does_not_reference_is_an_error_that_says_why()
        {
            // nameof leaves no reference in the assembly, so the stubs lack GoldCost and CSharp.lua cannot compile the map.
            var copy = folders.TemporaryCopyOfSample();
            File.AppendAllText(Path.Combine(copy, "Objects.cs"), """

                namespace Sample
                {
                    public static class Names
                    {
                        public static readonly string Life = nameof(Moonwell.Objects.Human.Units.Paladin.GoldCost);
                    }
                }
                """);

            var error = Assert.Throws<BuildException>(() => MapBuild.Run(copy, new BuildOptions(), new StringWriter()));

            Assert.Contains("GoldCost", error.Message);
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
            File.WriteAllText(project, Regex.Replace(text, @"[ 	]*<ProjectReference [^>]*Moonwell\.Objects\.csproj"" />
?
", string.Empty));

            // The library an earlier build of the project, when it still used Moonwell.Objects, left in its output.
            var output = Path.Combine(copy, "bin", "Debug", "net10.0");
            Directory.CreateDirectory(output);
            File.Copy(Path.Combine(builds.Folder, "bin", "Debug", "net10.0", "Moonwell.Objects.dll"), Path.Combine(output, "Moonwell.Objects.dll"));

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

        // The staged unit file of a built copy of the Sample.
        private static War3Net.Build.Object.UnitObjectData ReadUnits(string copy)
        {
            using var reader = new BinaryReader(File.OpenRead(Path.Combine(copy, "build", "Sample", "war3map.w3u")), Encoding.UTF8);
            return reader.ReadUnitObjectData();
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
