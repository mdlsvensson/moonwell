using System.Text;
using War3Net.Build.Extensions;
using War3Net.Build.Object;

namespace Moonwell.Build.Tests
{
    [Collection("Sample")]
    public sealed class ObjectFilesTests : IClassFixture<DeclarationsTests.BuiltObjects>, IDisposable
    {
        private static readonly string[] Kinds = { "unit", "item", "destructable", "doodad", "ability", "buff", "upgrade" };
        private static readonly string[] Extensions = { "w3u", "w3t", "w3b", "w3d", "w3a", "w3h", "w3q" };

        private readonly DeclarationsTests.BuiltObjects built;
        private readonly string folder = Path.Combine(Path.GetTempPath(), "moonwell-objectfiles-" + Guid.NewGuid().ToString("N"));
        private readonly StringWriter log = new StringWriter { NewLine = "\n" };

        public ObjectFilesTests(DeclarationsTests.BuiltObjects built)
        {
            this.built = built;
            Directory.CreateDirectory(Path.Combine(folder, "map"));
            File.WriteAllBytes(Path.Combine(folder, "map", "war3map.w3i"), Array.Empty<byte>());
            File.WriteAllText(Path.Combine(folder, "Sample.csproj"), "<Project />");
        }

        public void Dispose()
        {
            Directory.Delete(folder, true);
        }

        [Fact]
        public void world_editor_files_round_trip()
        {
            var fixture = TestFolders.Fixture("WorldEditorObjects");
            foreach (var extension in Extensions)
            {
                File.Copy(Path.Combine(fixture, "war3map." + extension), Path.Combine(folder, "map", "war3map." + extension));
                File.Copy(Path.Combine(fixture, "war3mapSkin." + extension), Path.Combine(folder, "map", "war3mapSkin." + extension));
            }

            var staged = WriteObjects(Array.Empty<DeclaredObject>(), new Dictionary<string, string>(), new Dictionary<string, string>());

            foreach (var extension in Extensions)
            {
                foreach (var name in new[] { "war3map." + extension, "war3mapSkin." + extension })
                {
                    Assert.Equal(File.ReadAllBytes(Path.Combine(fixture, name)), File.ReadAllBytes(Path.Combine(staged, name)));
                }
            }
        }

        [Fact]
        public void a_file_the_source_lacks_and_no_object_needs_is_not_written()
        {
            var staged = WriteObjects(Array.Empty<DeclaredObject>(), new Dictionary<string, string>(), new Dictionary<string, string>());

            Assert.Empty(Directory.GetFiles(staged));
        }

        [Fact]
        public void a_new_unit_is_the_world_editor_bytes()
        {
            // World Editor's save of a new unit based on the peasant with only a name: the skin file holds the name, the main file the unit with no modifications.
            var fixture = TestFolders.Fixture("WorldEditorObjects");
            var peasant = new DeclaredObject(
                "Objects.Peasant",
                "unit",
                "hpea",
                false,
                null,
                null,
                new SortedDictionary<string, object?> { ["unam"] = "TRIGSTR_012" },
                Array.Empty<string>());

            var staged = WriteObjects(new[] { peasant }, new Dictionary<string, string> { ["Objects.Peasant"] = "h000" }, new Dictionary<string, string>());

            Assert.Equal(File.ReadAllBytes(Path.Combine(fixture, "war3map.w3u")), File.ReadAllBytes(Path.Combine(staged, "war3map.w3u")));
            Assert.Equal(File.ReadAllBytes(Path.Combine(fixture, "war3mapSkin.w3u")), File.ReadAllBytes(Path.Combine(staged, "war3mapSkin.w3u")));
        }

        [Fact]
        public void a_new_ability_level_is_the_world_editor_bytes()
        {
            // World Editor's save of a new ability with a name: level 0, data pointer 0, in the skin file.
            var fixture = TestFolders.Fixture("WorldEditorObjects");
            var ability = new DeclaredObject(
                "Objects.Ability",
                "ability",
                "ANab",
                false,
                null,
                null,
                new SortedDictionary<string, object?> { ["anam"] = "TRIGSTR_016" },
                Array.Empty<string>());

            var staged = WriteObjects(new[] { ability }, new Dictionary<string, string> { ["Objects.Ability"] = "A000" }, new Dictionary<string, string>());

            Assert.Equal(File.ReadAllBytes(Path.Combine(fixture, "war3map.w3a")), File.ReadAllBytes(Path.Combine(staged, "war3map.w3a")));
            Assert.Equal(File.ReadAllBytes(Path.Combine(fixture, "war3mapSkin.w3a")), File.ReadAllBytes(Path.Combine(staged, "war3mapSkin.w3a")));
        }

        [Fact]
        public void arthas_written()
        {
            var staged = StageFixtureObjects(out var ids, out _);

            var main = ReadUnits(staged, "war3map.w3u");
            var skin = ReadUnits(staged, "war3mapSkin.w3u");
            var arthas = Assert.Single(main.NewUnits, one => one.NewId == IdOf(ids["Objects.Arthas"]));
            var arthasSkin = Assert.Single(skin.NewUnits, one => one.NewId == IdOf(ids["Objects.Arthas"]));
            Assert.Equal("H001", ids["Objects.Arthas"]);
            Assert.Equal(IdOf("Hpal"), arthas.OldId);
            Assert.Equal(IdOf("Hpal"), arthasSkin.OldId);
            Assert.Equal(new[] { 0 }, arthas.Unk);

            var hitPoints = Assert.Single(arthas.Modifications, one => one.Id == IdOf("uhpm"));
            Assert.Equal(ObjectDataType.Int, hitPoints.Type);
            Assert.Equal(2 * int.Parse(built.Standard.Value("Hpal", "uhpm", 0, null)!), hitPoints.Value);
            Assert.DoesNotContain(arthas.Modifications, one => one.Id == IdOf("unam"));

            var name = Assert.Single(arthasSkin.Modifications, one => one.Id == IdOf("unam"));
            Assert.Equal(ObjectDataType.String, name.Type);
            Assert.Equal("Arthas", name.Value);
            Assert.DoesNotContain(arthasSkin.Modifications, one => one.Id == IdOf("uhpm"));
        }

        [Fact]
        public void lists_references_and_flags_become_ids_and_numbers()
        {
            var staged = StageFixtureObjects(out var ids, out _);

            var main = ReadUnits(staged, "war3map.w3u");
            var arthas = Assert.Single(main.NewUnits, one => one.NewId == IdOf(ids["Objects.Arthas"]));
            Assert.Equal("AInv," + ids["Objects.Firebolt"], Assert.Single(arthas.Modifications, one => one.Id == IdOf("uabi")).Value);
            Assert.Equal("AHhb,AHds,AHre", Assert.Single(arthas.Modifications, one => one.Id == IdOf("uhab")).Value);

            var duelist = Assert.Single(main.NewUnits, one => one.NewId == IdOf(ids["Objects.Duelist"]));
            var reference = Assert.Single(duelist.Modifications, one => one.Id == IdOf("udaa"));
            Assert.Equal(ObjectDataType.String, reference.Type);
            Assert.Equal(ids["Objects.Firebolt"], reference.Value);
            var building = Assert.Single(duelist.Modifications, one => one.Id == IdOf("ubdg"));
            Assert.Equal(ObjectDataType.Int, building.Type);
            Assert.Equal(1, building.Value);
        }

        [Fact]
        public void chain_writes_standard_root()
        {
            var staged = StageFixtureObjects(out var ids, out _);

            var main = ReadUnits(staged, "war3map.w3u");
            var skin = ReadUnits(staged, "war3mapSkin.w3u");
            var dark = Assert.Single(main.NewUnits, one => one.NewId == IdOf(ids["Objects.DarkArthas"]));
            var darkSkin = Assert.Single(skin.NewUnits, one => one.NewId == IdOf(ids["Objects.DarkArthas"]));
            Assert.Equal(IdOf("Hpal"), dark.OldId);
            Assert.Equal("Dark Arthas", Assert.Single(darkSkin.Modifications, one => one.Id == IdOf("unam")).Value);
            Assert.Contains(dark.Modifications, one => one.Id == IdOf("uhpm"));
            Assert.NotEqual(ids["Objects.Arthas"], ids["Objects.DarkArthas"]);
        }

        [Fact]
        public void replaces_in_original_table()
        {
            var staged = StageFixtureObjects(out var ids, out _);

            var main = ReadUnits(staged, "war3map.w3u");
            var skin = ReadUnits(staged, "war3mapSkin.w3u");
            Assert.Equal("hfoo", ids["Objects.StrongFootman"]);
            var footman = Assert.Single(main.BaseUnits);
            Assert.Equal(IdOf("hfoo"), footman.OldId);
            Assert.Equal(0, footman.NewId);
            Assert.Equal(600, Assert.Single(footman.Modifications, one => one.Id == IdOf("uhpm")).Value);
            Assert.Single(skin.BaseUnits);
            Assert.DoesNotContain(main.NewUnits, one => one.OldId == IdOf("hfoo") && one.Modifications.Any(mod => mod.Id == IdOf("uhpm") && Equals(mod.Value, 600)));
        }

        [Fact]
        public void ability_levels_written()
        {
            var staged = StageFixtureObjects(out var ids, out _);

            var skin = ReadAbilities(staged, "war3mapSkin.w3a");
            var abilities = ReadAbilities(staged, "war3map.w3a");
            var firebolt = Assert.Single(abilities.NewAbilities, one => one.NewId == IdOf(ids["Objects.Firebolt"]));
            Assert.Equal(IdOf("AHtb"), firebolt.OldId);
            var damage = firebolt.Modifications.Where(one => one.Id == IdOf("Htb1")).OrderBy(one => one.Level).ToList();
            Assert.Equal(new[] { 1, 2, 3 }, damage.Select(one => one.Level));
            Assert.All(damage, one => Assert.Equal(1, one.Pointer));
            Assert.All(damage, one => Assert.Equal(ObjectDataType.Unreal, one.Type));
            Assert.Equal(new object[] { 100f, 200f, 300f }, damage.Select(one => one.Value));

            var tuned = Assert.Single(abilities.NewAbilities, one => one.NewId == IdOf(ids["Objects.Tuned"]));
            var tunedDamage = tuned.Modifications.Where(one => one.Id == IdOf("Htb1")).OrderBy(one => one.Level).ToList();
            Assert.Equal(new object[] { 100f, 250f, 300f }, tunedDamage.Select(one => one.Value));

            // The name is on the skin side, level 0 and pointer 0, and the tooltip has a value for each level.
            var names = Assert.Single(skin.NewAbilities, one => one.NewId == IdOf(ids["Objects.Firebolt"]));
            var name = Assert.Single(names.Modifications, one => one.Id == IdOf("anam"));
            Assert.Equal("Firebolt", name.Value);
            Assert.Equal(0, name.Level);
            Assert.Equal(0, name.Pointer);
        }

        [Fact]
        public void buff_and_carrier()
        {
            var staged = StageFixtureObjects(out var ids, out var set);

            var burn = set.Objects.Single(one => one.ClassName == "Objects.Burn");
            var buffs = ReadBuffs(staged, "war3mapSkin.w3h");
            var buff = Assert.Single(buffs.NewBuffs, one => one.NewId == IdOf(ids["Objects.Burn"]));
            Assert.Equal(IdOf(burn.StandardRoot), buff.OldId);
            Assert.Equal("Burn", Assert.Single(buff.Modifications, one => one.Id == IdOf("fnam")).Value);

            var abilities = ReadAbilities(staged, "war3map.w3a");
            var carrier = Assert.Single(abilities.NewAbilities, one => one.NewId == IdOf(ids["Objects.Burn#carrier"]));
            Assert.Equal(IdOf("Aasl"), carrier.OldId);
            var applied = Assert.Single(carrier.Modifications, one => one.Id == IdOf("abuf"));
            Assert.Equal(1, applied.Level);
            Assert.Equal(ids["Objects.Burn"], applied.Value);
            var target = Assert.Single(carrier.Modifications, one => one.Id == IdOf("atar"));
            Assert.Equal(1, target.Level);
            Assert.Equal("self", target.Value);
            var area = Assert.Single(carrier.Modifications, one => one.Id == IdOf("aare"));
            Assert.Equal(1, area.Level);
            Assert.Equal(1f, area.Value);
            Assert.Equal(ObjectDataType.Unreal, area.Type);
        }

        [Fact]
        public void a_replaced_buff_has_a_carrier()
        {
            var buff = new DeclaredObject(
                "Objects.Hot",
                "buff",
                "BEim",
                true,
                null,
                null,
                new SortedDictionary<string, object?> { ["fnam"] = "Hot" },
                Array.Empty<string>());
            var ids = new Dictionary<string, string> { ["Objects.Hot"] = "BEim", ["Objects.Hot#carrier"] = "A001" };

            var staged = WriteObjects(new[] { buff }, ids, new Dictionary<string, string>(), built.Standard);

            var buffs = ReadBuffs(staged, "war3mapSkin.w3h");
            Assert.Equal(IdOf("BEim"), Assert.Single(buffs.BaseBuffs).OldId);
            var carrier = Assert.Single(ReadAbilities(staged, "war3map.w3a").NewAbilities);
            Assert.Equal(IdOf("A001"), carrier.NewId);
            Assert.Equal("BEim", Assert.Single(carrier.Modifications, one => one.Id == IdOf("abuf")).Value);
        }

        [Fact]
        public void every_kind_goes_to_its_files()
        {
            var staged = StageFixtureObjects(out var ids, out _);

            Assert.Equal(IdOf(ids["Objects.Claws"]), Assert.Single(ReadItems(staged, "war3map.w3t").NewItems).NewId);
            Assert.Equal(900, Assert.Single(Assert.Single(ReadItems(staged, "war3map.w3t").NewItems).Modifications, one => one.Id == IdOf("igol")).Value);
            Assert.Equal(IdOf(ids["Objects.Gate"]), Assert.Single(ReadDestructables(staged, "war3map.w3b").NewDestructables).NewId);
            Assert.Equal("Gate", Assert.Single(Assert.Single(ReadDestructables(staged, "war3mapSkin.w3b").NewDestructables).Modifications, one => one.Id == IdOf("bnam")).Value);
            Assert.Equal(IdOf(ids["Objects.Rock"]), Assert.Single(ReadDoodads(staged, "war3map.w3d").NewDoodads).NewId);
            Assert.Equal("Rock", Assert.Single(Assert.Single(ReadDoodads(staged, "war3map.w3d").NewDoodads).Modifications, one => one.Id == IdOf("dnam")).Value);

            var forge = Assert.Single(ReadUpgrades(staged, "war3mapSkin.w3q").NewUpgrades);
            Assert.Equal(IdOf(ids["Objects.Forge"]), forge.NewId);
            var names = forge.Modifications.Where(one => one.Id == IdOf("gnam")).OrderBy(one => one.Level).ToList();
            Assert.Equal(new[] { 1, 2, 3 }, names.Select(one => one.Level));
            Assert.Equal(new object[] { "Forge 1", "Forge 2", "Forge 3" }, names.Select(one => one.Value));
        }

        [Fact]
        public void code_wins_on_matching_id()
        {
            // A saved built map: Arthas is in the source map already, with other values, and so is the replaced footman.
            var source = new UnitObjectData(ObjectDataFormatVersion.v3);
            source.NewUnits.Add(NewUnit("Hpal", "H001", Mod("uhpm", 1)));
            source.BaseUnits.Add(NewUnit("hfoo", string.Empty, Mod("uhpm", 1)));
            var skinSource = new UnitObjectData(ObjectDataFormatVersion.v3);
            skinSource.NewUnits.Add(NewUnit("Hpal", "H001", Mod("unam", "Old name")));
            skinSource.BaseUnits.Add(NewUnit("hfoo", string.Empty));
            WriteSource("war3map.w3u", source);
            WriteSource("war3mapSkin.w3u", skinSource);

            var staged = StageFixtureObjects(out var ids, out _);

            Assert.Equal("H001", ids["Objects.Arthas"]);
            var main = ReadUnits(staged, "war3map.w3u");
            var skin = ReadUnits(staged, "war3mapSkin.w3u");
            var arthas = Assert.Single(main.NewUnits, one => one.NewId == IdOf("H001"));
            Assert.Equal(IdOf("Hpal"), arthas.OldId);
            Assert.NotEqual(1, Assert.Single(arthas.Modifications, one => one.Id == IdOf("uhpm")).Value);
            Assert.Equal("Arthas", Assert.Single(Assert.Single(skin.NewUnits, one => one.NewId == IdOf("H001")).Modifications).Value);
            Assert.Equal(600, Assert.Single(Assert.Single(main.BaseUnits).Modifications, one => one.Id == IdOf("uhpm")).Value);
            Assert.DoesNotContain("warning:", log.ToString());
        }

        [Fact]
        public void a_declared_object_replacing_another_base_is_warned_about()
        {
            // World Editor's H001 is a footman; the class with H001 is a paladin.
            var source = new UnitObjectData(ObjectDataFormatVersion.v3);
            source.NewUnits.Add(NewUnit("hfoo", "H001", Mod("uhpm", 1)));
            var skinSource = new UnitObjectData(ObjectDataFormatVersion.v3);
            skinSource.NewUnits.Add(NewUnit("hfoo", "H001", Mod("unam", "Mine")));
            WriteSource("war3map.w3u", source);
            WriteSource("war3mapSkin.w3u", skinSource);
            var arthas = new DeclaredObject("Objects.Arthas", "hero", "Hpal", false, null, null, new SortedDictionary<string, object?>(), Array.Empty<string>());

            var staged = WriteObjects(new[] { arthas }, new Dictionary<string, string> { ["Objects.Arthas"] = "H001" }, new Dictionary<string, string>());

            Assert.Equal(@"warning: map\war3map.w3u's H001 (based on hfoo) is replaced by Objects.Arthas (based on Hpal)" + "\n", log.ToString());
            Assert.Equal(IdOf("Hpal"), Assert.Single(ReadUnits(staged, "war3map.w3u").NewUnits).OldId);
        }

        [Fact]
        public void a_declared_object_replacing_the_same_base_is_silent()
        {
            var source = new UnitObjectData(ObjectDataFormatVersion.v3);
            source.NewUnits.Add(NewUnit("Hpal", "H001", Mod("uhpm", 1)));
            WriteSource("war3map.w3u", source);
            var arthas = new DeclaredObject("Objects.Arthas", "hero", "Hpal", false, null, null, new SortedDictionary<string, object?>(), Array.Empty<string>());

            WriteObjects(new[] { arthas }, new Dictionary<string, string> { ["Objects.Arthas"] = "H001" }, new Dictionary<string, string>());

            Assert.Empty(log.ToString());
        }

        [Fact]
        public void undeclared_kept_with_warning()
        {
            // World Editor made h000 and changed the standard peasant; no class declares either.
            var source = new UnitObjectData(ObjectDataFormatVersion.v3);
            source.NewUnits.Add(NewUnit("hpea", "h000"));
            source.BaseUnits.Add(NewUnit("hpea", string.Empty, Mod("uhpm", 5)));
            var skinSource = new UnitObjectData(ObjectDataFormatVersion.v3);
            skinSource.NewUnits.Add(NewUnit("hpea", "h000", Mod("unam", "Mine")));
            skinSource.BaseUnits.Add(NewUnit("hpea", string.Empty));
            WriteSource("war3map.w3u", source);
            WriteSource("war3mapSkin.w3u", skinSource);

            var staged = StageFixtureObjects(out _, out _);

            var main = ReadUnits(staged, "war3map.w3u");
            Assert.Contains(main.NewUnits, one => one.NewId == IdOf("h000"));
            Assert.Contains(main.BaseUnits, one => one.OldId == IdOf("hpea"));
            Assert.Equal(@"warning: map\war3map.w3u keeps h000, which no class declares" + "\n", log.ToString());
        }

        [Fact]
        public void undeclared_kept_in_the_skin_file_only_is_named_by_it()
        {
            var skinSource = new UnitObjectData(ObjectDataFormatVersion.v3);
            skinSource.NewUnits.Add(NewUnit("hpea", "h000", Mod("unam", "Mine")));
            WriteSource("war3mapSkin.w3u", skinSource);

            StageFixtureObjects(out _, out _);

            Assert.Equal(@"warning: map\war3mapSkin.w3u keeps h000, which no class declares" + "\n", log.ToString());
        }

        [Fact]
        public void rebuild_from_built_map_is_stable()
        {
            var staged = StageFixtureObjects(out var ids, out var set);
            var firstBytes = ObjectFileBytes(staged);
            Assert.NotEmpty(firstBytes);
            Assert.Empty(log.ToString());

            // The staged object files become the source map's, as when a modder saves the built map.
            foreach (var name in firstBytes.Keys)
            {
                File.Copy(Path.Combine(staged, name), Path.Combine(folder, "map", name), true);
            }

            var project = MapProject.Find(folder);
            var again = ObjectIds.Assign(set.Objects, AllStandardIds(), new HashSet<string>(ids.Values));
            var stagedAgain = WriteObjects(set.Objects, again, set.StandardClassIds, built.Standard);

            Assert.Equal(ids, again);
            var secondBytes = ObjectFileBytes(stagedAgain);
            Assert.Equal(firstBytes.Keys, secondBytes.Keys);
            foreach (var name in firstBytes.Keys)
            {
                Assert.Equal(firstBytes[name], secondBytes[name]);
            }

            Assert.Empty(log.ToString());
        }

        [Fact]
        public void a_class_with_no_object_in_a_list_fails()
        {
            var unit = new DeclaredObject(
                "Objects.Odd",
                "unit",
                "hfoo",
                false,
                null,
                null,
                new SortedDictionary<string, object?> { ["uabi"] = new[] { "Objects.Missing" } },
                new[] { "Objects.Missing" });

            var error = Assert.Throws<BuildException>(
                () => WriteObjects(new[] { unit }, new Dictionary<string, string> { ["Objects.Odd"] = "h001" }, new Dictionary<string, string>(), built.Standard));

            Assert.Equal("Objects.Odd: uabi names Objects.Missing, which no object declares", error.Message);
        }

        [Fact]
        public void an_id_with_no_standard_class_is_written_as_it_is()
        {
            var unit = new DeclaredObject(
                "Objects.Odd",
                "unit",
                "hfoo",
                false,
                null,
                null,
                new SortedDictionary<string, object?> { ["uabi"] = new[] { "Adef", "Objects.Odd" } },
                new[] { "Objects.Odd" });

            var staged = WriteObjects(new[] { unit }, new Dictionary<string, string> { ["Objects.Odd"] = "h001" }, new Dictionary<string, string>(), built.Standard);

            var odd = Assert.Single(ReadUnits(staged, "war3map.w3u").NewUnits);
            Assert.Equal("Adef,h001", Assert.Single(odd.Modifications).Value);
        }

        [Fact]
        public void a_word_in_a_number_field_is_parsed()
        {
            var unit = new DeclaredObject(
                "Objects.Odd",
                "unit",
                "hfoo",
                false,
                null,
                null,
                new SortedDictionary<string, object?> { ["ugol"] = "2" },
                Array.Empty<string>());

            var staged = WriteObjects(new[] { unit }, new Dictionary<string, string> { ["Objects.Odd"] = "h001" }, new Dictionary<string, string>(), built.Standard);

            var mod = Assert.Single(Assert.Single(ReadUnits(staged, "war3map.w3u").NewUnits).Modifications);
            Assert.Equal(ObjectDataType.Int, mod.Type);
            Assert.Equal(2, mod.Value);
        }

        [Fact]
        public void a_word_that_is_no_number_fails()
        {
            var unit = new DeclaredObject(
                "Objects.Odd",
                "unit",
                "hfoo",
                false,
                null,
                null,
                new SortedDictionary<string, object?> { ["ugol"] = "some" },
                Array.Empty<string>());

            var error = Assert.Throws<BuildException>(
                () => WriteObjects(new[] { unit }, new Dictionary<string, string> { ["Objects.Odd"] = "h001" }, new Dictionary<string, string>(), built.Standard));

            Assert.Equal("Objects.Odd: ugol is a whole number in the object file, and \"some\" is not one", error.Message);
        }

        [Fact]
        public void choices_are_written_as_the_game_spells_them()
        {
            var staged = StageFixtureObjects(out var ids, out _);

            var mage = Assert.Single(ReadUnits(staged, "war3map.w3u").NewUnits, one => one.NewId == IdOf(ids["Objects.Mage"]));
            var attack = Assert.Single(mage.Modifications, one => one.Id == IdOf("ua1t"));
            Assert.Equal(ObjectDataType.String, attack.Type);
            Assert.Equal("magic", attack.Value);
            var targets = Assert.Single(mage.Modifications, one => one.Id == IdOf("ua1g"));
            Assert.Equal(ObjectDataType.String, targets.Type);
            Assert.Equal("air,enemies", targets.Value);
            var death = Assert.Single(mage.Modifications, one => one.Id == IdOf("udea"));
            Assert.Equal(ObjectDataType.Int, death.Type);
            Assert.Equal(1, death.Value);
            Assert.DoesNotContain(mage.Modifications, one => one.Id == IdOf("umvt") || one.Id == IdOf("utar"));

            var zap = Assert.Single(ReadAbilities(staged, "war3map.w3a").NewAbilities, one => one.NewId == IdOf(ids["Objects.Zap"]));
            Assert.Equal(
                new[] { (1, "air,enemies"), (2, "ground"), (3, "ground,structure") },
                zap.Modifications.Where(one => one.Id == IdOf("atar")).Select(one => (one.Level, (string)one.Value)).ToArray());
        }

        [Fact]
        public void a_choice_given_as_the_games_text_is_written_as_world_editor_spells_it()
        {
            var unit = Odd(new SortedDictionary<string, object?> { ["ua1t"] = "MAGIC", ["ua1g"] = "enemy,aliv", ["udea"] = "2" });

            var staged = WriteObjects(new[] { unit }, new Dictionary<string, string> { ["Objects.Odd"] = "h001" }, new Dictionary<string, string>(), built.Standard);

            var mods = Assert.Single(ReadUnits(staged, "war3map.w3u").NewUnits).Modifications;
            Assert.Equal("magic", Assert.Single(mods, one => one.Id == IdOf("ua1t")).Value);
            Assert.Equal("alive,enemies", Assert.Single(mods, one => one.Id == IdOf("ua1g")).Value);
            Assert.Equal(2, Assert.Single(mods, one => one.Id == IdOf("udea")).Value);
        }

        [Theory]
        [InlineData("ua1t", 42, "Objects.Odd: ua1t is 42, which is no value of AttackType")]
        [InlineData("upap", 64, "Objects.Odd: upap is 64, whose bit 64 is no value of PathingListPrevent")]
        [InlineData("ua1t", "sharp", "Objects.Odd: ua1t holds an AttackType, and \"sharp\" is none of its values")]
        public void a_choice_that_is_no_value_fails(string field, object value, string message)
        {
            var unit = Odd(new SortedDictionary<string, object?> { [field] = value });

            var error = Assert.Throws<BuildException>(
                () => WriteObjects(new[] { unit }, new Dictionary<string, string> { ["Objects.Odd"] = "h001" }, new Dictionary<string, string>(), built.Standard));

            Assert.Equal(message, error.Message);
        }

        [Fact]
        public void a_real_goes_in_its_real_store()
        {
            var item = new DeclaredObject(
                "Objects.Big",
                "item",
                "ratf",
                false,
                null,
                null,
                new SortedDictionary<string, object?> { ["isca"] = 2f },
                Array.Empty<string>());

            var staged = WriteObjects(new[] { item }, new Dictionary<string, string> { ["Objects.Big"] = "I001" }, new Dictionary<string, string>(), built.Standard);

            var mod = Assert.Single(Assert.Single(ReadItems(staged, "war3mapSkin.w3t").NewItems).Modifications);
            Assert.Equal(ObjectDataType.Real, mod.Type);
            Assert.Equal(2f, mod.Value);
        }

        private static DeclaredObject Odd(SortedDictionary<string, object?> changed)
        {
            return new DeclaredObject("Objects.Odd", "unit", "hfoo", false, null, null, changed, Array.Empty<string>());
        }

        private string StageFixtureObjects(out IReadOnlyDictionary<string, string> ids, out DeclarationSet set)
        {
            set = built.Read(null);
            var project = MapProject.Find(folder);
            ids = ObjectIds.Assign(set.Objects, AllStandardIds(), new HashSet<string>());
            return WriteObjects(set.Objects, ids, set.StandardClassIds, built.Standard);
        }

        private string WriteObjects(IReadOnlyList<DeclaredObject> objects, IReadOnlyDictionary<string, string> ids, IReadOnlyDictionary<string, string> standardClassIds)
        {
            return WriteObjects(objects, ids, standardClassIds, built.Standard);
        }

        private string WriteObjects(IReadOnlyList<DeclaredObject> objects, IReadOnlyDictionary<string, string> ids, IReadOnlyDictionary<string, string> standardClassIds, StandardData standard)
        {
            var project = MapProject.Find(folder);
            var staged = Path.Combine(folder, "staged-" + Guid.NewGuid().ToString("N"));
            ObjectFiles.Write(project, objects, ids, standardClassIds, standard, staged, log);
            return staged;
        }

        private IReadOnlySet<string> AllStandardIds()
        {
            var all = new HashSet<string>();
            foreach (var kind in Kinds)
            {
                all.UnionWith(built.Standard.Ids(kind));
            }

            return all;
        }

        private SortedDictionary<string, byte[]> ObjectFileBytes(string stagedFolder)
        {
            var files = new SortedDictionary<string, byte[]>(StringComparer.Ordinal);
            foreach (var extension in Extensions)
            {
                foreach (var name in new[] { "war3map." + extension, "war3mapSkin." + extension })
                {
                    var path = Path.Combine(stagedFolder, name);
                    if (File.Exists(path))
                    {
                        files[name] = File.ReadAllBytes(path);
                    }
                }
            }

            return files;
        }

        private void WriteSource(string name, UnitObjectData data)
        {
            using var stream = File.Create(Path.Combine(folder, "map", name));
            using var writer = new BinaryWriter(stream, Encoding.UTF8);
            writer.Write(data);
        }

        private static SimpleObjectModification NewUnit(string oldId, string newId, params SimpleObjectDataModification[] modifications)
        {
            var one = new SimpleObjectModification { OldId = IdOf(oldId), NewId = newId.Length == 0 ? 0 : IdOf(newId) };
            one.Unk.Add(0);
            one.Modifications.AddRange(modifications);
            return one;
        }

        private static SimpleObjectDataModification Mod(string field, object value)
        {
            return new SimpleObjectDataModification
            {
                Id = IdOf(field),
                Type = value is int ? ObjectDataType.Int : ObjectDataType.String,
                Value = value,
            };
        }

        private static int IdOf(string text)
        {
            return BitConverter.ToInt32(Encoding.ASCII.GetBytes(text));
        }

        private static UnitObjectData ReadUnits(string staged, string name)
        {
            using var reader = Open(staged, name);
            return reader.ReadUnitObjectData();
        }

        private static ItemObjectData ReadItems(string staged, string name)
        {
            using var reader = Open(staged, name);
            return reader.ReadItemObjectData();
        }

        private static DestructableObjectData ReadDestructables(string staged, string name)
        {
            using var reader = Open(staged, name);
            return reader.ReadDestructableObjectData();
        }

        private static DoodadObjectData ReadDoodads(string staged, string name)
        {
            using var reader = Open(staged, name);
            return reader.ReadDoodadObjectData();
        }

        private static AbilityObjectData ReadAbilities(string staged, string name)
        {
            using var reader = Open(staged, name);
            return reader.ReadAbilityObjectData();
        }

        private static BuffObjectData ReadBuffs(string staged, string name)
        {
            using var reader = Open(staged, name);
            return reader.ReadBuffObjectData();
        }

        private static UpgradeObjectData ReadUpgrades(string staged, string name)
        {
            using var reader = Open(staged, name);
            return reader.ReadUpgradeObjectData();
        }

        private static BinaryReader Open(string staged, string name)
        {
            return new BinaryReader(File.OpenRead(Path.Combine(staged, name)), Encoding.UTF8);
        }
    }
}
