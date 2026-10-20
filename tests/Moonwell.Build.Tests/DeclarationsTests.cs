using Xunit;

namespace Moonwell.Build.Tests
{
    [Collection("Sample")]
    public sealed class DeclarationsTests : IClassFixture<DeclarationsTests.BuiltObjects>
    {
        private readonly BuiltObjects built;

        public DeclarationsTests(BuiltObjects built)
        {
            this.built = built;
        }

        [Fact]
        public void reads_the_carrier_of_a_buff()
        {
            var burn = Find(built.Read(null).Objects, "Objects.Burn");

            Assert.Equal("B001", burn.PinnedId);
            Assert.Equal("A005", burn.CarrierId);
        }

        [Fact]
        public void reads_arthas()
        {
            var arthas = Find(built.Read(null).Objects, "Objects.Arthas");

            Assert.Equal("hero", arthas.Kind);
            Assert.Equal("Hpal", arthas.StandardRoot);
            Assert.False(arthas.Replaces);
            Assert.Equal("H001", arthas.PinnedId);
            Assert.Null(arthas.CarrierId);
            Assert.Equal(new[] { "uabi", "ugol", "uhab", "uhpm", "unam" }, arthas.Changed.Keys);
            Assert.Equal("Arthas", arthas.Changed["unam"]);
            Assert.Equal(2 * BaseInt("Hpal", "uhpm"), arthas.Changed["uhpm"]);
            Assert.Equal(2 * BaseInt("Hpal", "ugol"), arthas.Changed["ugol"]);
            Assert.Equal(
                new[] { "Moonwell.Objects.Other.Abilities.Inventory", "Objects.Firebolt" },
                Assert.IsAssignableFrom<IReadOnlyList<string>>(arthas.Changed["uabi"]));
            // The Paladin's hero abilities without the one Arthas removed; it stands in the standard list before.
            var heroAbilities = Assert.IsAssignableFrom<IReadOnlyList<string>>(arthas.Changed["uhab"]);
            Assert.Equal(
                new[]
                {
                    "Moonwell.Objects.Human.Abilities.HolyLight",
                    "Moonwell.Objects.Human.Abilities.DivineShield",
                    "Moonwell.Objects.Human.Abilities.Resurrection",
                },
                heroAbilities);
            Assert.Equal(
                new[]
                {
                    "Moonwell.Objects.Human.Abilities.DivineShield",
                    "Moonwell.Objects.Human.Abilities.HolyLight",
                    "Moonwell.Objects.Human.Abilities.Resurrection",
                    "Moonwell.Objects.Other.Abilities.Inventory",
                    "Objects.Firebolt",
                },
                arthas.References);
        }

        [Fact]
        public void contains_and_add_see_the_standard_list()
        {
            var duelist = Find(built.Read(null).Objects, "Objects.Duelist");

            // Contains<Defend> was true in the constructor; Footman's list is Defend and Aihn, then Firebolt (added in a lambda).
            Assert.Equal("Defender", duelist.Changed["unam"]);
            var abilities = Assert.IsAssignableFrom<IReadOnlyList<string>>(duelist.Changed["uabi"]);
            Assert.Equal(3, abilities.Count);
            Assert.Equal("Moonwell.Objects.Human.Abilities.Defend", abilities[0]);
            Assert.Equal("Objects.Firebolt", abilities[2]);
        }

        [Fact]
        public void a_reference_and_a_bool_are_read()
        {
            var duelist = Find(built.Read(null).Objects, "Objects.Duelist");

            Assert.Equal("Objects.Firebolt", duelist.Changed["udaa"]);
            Assert.Equal(true, duelist.Changed["ubdg"]);
            Assert.Contains("Objects.Firebolt", duelist.References);
        }

        [Fact]
        public void a_reference_of_each_level()
        {
            var elemental = Find(built.Read(null).Objects, "Objects.Elemental");

            Assert.Equal("ability", elemental.Kind);
            Assert.Equal("AHwe", elemental.StandardRoot);
            Assert.Equal(new[] { "Hwe1:1", "Hwe1:2", "Hwe1:3" }, elemental.Changed.Keys);
            Assert.All(elemental.Changed.Values, value => Assert.Equal("Objects.Arthas", value));
            Assert.Equal(new[] { "Objects.Arthas" }, elemental.References);
        }

        [Fact]
        public void a_list_of_each_level_is_one_list()
        {
            var portal = Find(built.Read(null).Objects, "Objects.Portal");

            Assert.Equal(new[] { "Ndp1:1", "Ndp1:2", "Ndp1:3" }, portal.Changed.Keys);
            foreach (var value in portal.Changed.Values)
            {
                var list = Assert.IsAssignableFrom<IReadOnlyList<string>>(value);
                Assert.Equal(3, list.Count);
                Assert.Equal("Objects.Arthas", list[2]);
                Assert.Equal(portal.Changed["Ndp1:1"], value);
            }
        }

        [Fact]
        public void a_level_put_by_itself_wins_over_the_whole_field()
        {
            var tuned = Find(built.Read(null).Objects, "Objects.Tuned");

            Assert.Equal(100f, tuned.Changed["Htb1:1"]);
            Assert.Equal(250f, tuned.Changed["Htb1:2"]);
            Assert.Equal(300f, tuned.Changed["Htb1:3"]);
        }

        [Fact]
        public void unset_field_reads_chosen_set()
        {
            // Hpal's gold cost is 400 in the base tables and 500 in melee_v0.
            Assert.Equal("500", built.Standard.Value("Hpal", "ugol", 0, "melee_v0"));

            var arthas = Find(built.Read("melee_v0").Objects, "Objects.Arthas");

            Assert.Equal(1000, arthas.Changed["ugol"]);
            Assert.Equal(200, arthas.Changed["uhpm"]);
        }

        [Fact]
        public void chain_merges()
        {
            var arthas = Find(built.Read(null).Objects, "Objects.Arthas");
            var dark = Find(built.Read(null).Objects, "Objects.DarkArthas");

            Assert.Equal("Hpal", dark.StandardRoot);
            Assert.Equal("hero", dark.Kind);
            Assert.Equal("Dark Arthas", dark.Changed["unam"]);
            foreach (var key in new[] { "uabi", "ugol", "uhpm" })
            {
                Assert.Equal(arthas.Changed[key], dark.Changed[key]);
            }

            Assert.Equal(arthas.Changed.Keys, dark.Changed.Keys);
        }

        [Fact]
        public void per_level_values()
        {
            var firebolt = Find(built.Read(null).Objects, "Objects.Firebolt");

            Assert.Equal("ability", firebolt.Kind);
            Assert.Equal("AHtb", firebolt.StandardRoot);
            // Every level of a changed field is there, though level 1 and 2 equal the standard ones.
            Assert.Equal(100f, firebolt.Changed["Htb1:1"]);
            Assert.Equal(200f, firebolt.Changed["Htb1:2"]);
            Assert.Equal(300f, firebolt.Changed["Htb1:3"]);
            Assert.DoesNotContain("Htb1:4", firebolt.Changed.Keys);
            Assert.Equal("Firebolt", firebolt.Changed["anam"]);
            Assert.Equal("Deals 100 damage", firebolt.Changed["atp1:1"]);
            Assert.Equal("Deals 200 damage", firebolt.Changed["atp1:2"]);
            Assert.Equal("Deals 300 damage", firebolt.Changed["atp1:3"]);
        }

        [Fact]
        public void plain_buff_has_no_object()
        {
            var declared = built.Read(null).Objects;

            Assert.DoesNotContain(declared, one => one.ClassName == "Objects.Smoke");
            var burn = Find(declared, "Objects.Burn");
            Assert.Equal("buff", burn.Kind);
            Assert.Equal("BEim", burn.StandardRoot);
            Assert.Equal(new[] { "fnam" }, burn.Changed.Keys);
            Assert.Equal("Burn", burn.Changed["fnam"]);
        }

        [Fact]
        public void replaces_is_marked()
        {
            var declared = built.Read(null).Objects;

            var strong = Find(declared, "Objects.StrongFootman");
            Assert.True(strong.Replaces);
            Assert.Equal("unit", strong.Kind);
            Assert.Equal("hfoo", strong.StandardRoot);
            Assert.Equal(600, strong.Changed["uhpm"]);
            Assert.Single(declared, one => one.Replaces);
        }

        [Theory]
        [InlineData("Objects.Claws", "item", "ratf", "igol", 900)]
        [InlineData("Objects.Gate", "destructable", "ATg1", "bnam", "Gate")]
        [InlineData("Objects.Rock", "doodad", "BRrk", "dnam", "Rock")]
        [InlineData("Objects.Forge", "upgrade", "Rhme", "gnam:3", "Forge 3")]
        public void every_kind_is_read(string className, string kind, string root, string field, object value)
        {
            var declared = Find(built.Read(null).Objects, className);

            Assert.Equal(kind, declared.Kind);
            Assert.Equal(root, declared.StandardRoot);
            Assert.Equal(value, declared.Changed[field]);
        }

        [Fact]
        public void a_choice_is_kept_as_its_number_and_one_the_same_as_the_standard_is_not()
        {
            var objects = built.Read(null).Objects;

            var mage = Find(objects, "Objects.Mage");
            Assert.Equal(6, mage.Changed["ua1t"]);
            Assert.Equal(1 | 32, mage.Changed["ua1g"]);
            Assert.Equal(1, mage.Changed["udea"]);
            // The footman moves on foot and is targeted as ground: the mage's same choices are not changes.
            Assert.False(mage.Changed.ContainsKey("umvt"));
            Assert.False(mage.Changed.ContainsKey("utar"));

            var zap = Find(objects, "Objects.Zap");
            Assert.Equal(new object?[] { 1 | 32, 64, 64 | 1048576 }, new[] { zap.Changed["atar:1"], zap.Changed["atar:2"], zap.Changed["atar:3"] });
        }

        [Fact]
        public void ordered_by_class_name()
        {
            var names = built.Read(null).Objects.Select(one => one.ClassName).ToList();

            Assert.Equal(
                new[]
                {
                    "Objects.Arthas", "Objects.Burn", "Objects.Claws", "Objects.DarkArthas", "Objects.Drilled", "Objects.Duelist",
                    "Objects.Elemental", "Objects.Firebolt", "Objects.Forge", "Objects.Gate", "Objects.Mage", "Objects.Portal",
                    "Objects.Rock", "Objects.StrongFootman", "Objects.Tuned", "Objects.Zap",
                },
                names);
        }

        [Fact]
        public void standard_classes_named_by_lists_and_references_have_their_ids()
        {
            var ids = built.Read(null).StandardClassIds;

            Assert.Equal("AHhb", ids["Moonwell.Objects.Human.Abilities.HolyLight"]);
            Assert.Equal("AHds", ids["Moonwell.Objects.Human.Abilities.DivineShield"]);
            Assert.Equal("Adef", ids["Moonwell.Objects.Human.Abilities.Defend"]);
            Assert.DoesNotContain("Objects.Firebolt", ids.Keys);
            Assert.DoesNotContain("Moonwell.Objects.Human.Units.Paladin", ids.Keys);
        }

        [Fact]
        public void map_without_objects_declares_none()
        {
            var project = MapProject.Find(TestFolders.Fixture("CallsGame"));
            var assembly = DotnetBuild.Run(project, false, TextWriter.Null);

            Assert.Empty(Declarations.Read(assembly, project, built.Standard, null).Objects);
        }

        [Fact]
        public void standard_value_is_the_sets_else_the_base()
        {
            var standard = built.Standard;

            Assert.Equal("400", standard.Value("Hpal", "ugol", 0, null));
            Assert.Equal("500", standard.Value("Hpal", "ugol", 0, "melee_v0"));
            Assert.Equal("425", standard.Value("Hpal", "ugol", 0, "custom_v1"));
            // A set the object does not change reads the base.
            Assert.Equal("100", standard.Value("Hpal", "uhpm", 0, "melee_v0"));
            // The empty text in a set is a key that set has no value for, though the base has one.
            Assert.Equal("0", standard.Value("Hpal", "ua2b", 0, null));
            Assert.Null(standard.Value("Hpal", "ua2b", 0, "custom_v0"));
            Assert.Equal("225", standard.Value("AHtb", "Htb1", 2, "melee_v0"));
            Assert.Equal("100", standard.Value("AHtb", "Htb1", 1, "melee_v0"));
            Assert.Null(standard.Value("nope", "uhpm", 0, null));
        }

        [Theory]
        [InlineData(2, 2, "400")]
        [InlineData(0, 2, "400")]
        [InlineData(0, 1, "425")]
        [InlineData(1, 1, "425")]
        [InlineData(2, 1, "400")]
        [InlineData(2, 0, "500")]
        public void game_data_option_decides_the_tables_values_come_from(int dataSet, int dataVersion, string gold)
        {
            // Hpal's gold cost is 400 in the base tables, 425 in custom_v1 and 500 in melee_v0.
            var data = GameData.Of(dataSet, dataVersion);

            Assert.Equal(gold, built.Standard.Value("Hpal", "ugol", 0, data.ValueSet));
        }

        [Fact]
        public void melee_with_frozen_throne_has_base_values_and_the_objects_of_custom_v1()
        {
            var data = GameData.Of(2, 1);

            // Eevi (Illidan) is in custom_v1 and the base tables, not in melee_v0; the spur is in neither custom_v1 nor the base tables.
            // The Forsaken Apothecary (fapo) is in the base tables only, which tells custom_v1 apart from the base.
            Assert.False(built.Standard.IsMissing("Eevi", data.ExistenceSet));
            Assert.True(built.Standard.IsMissing("spur", data.ExistenceSet));
            Assert.True(built.Standard.IsMissing("fapo", data.ExistenceSet));
            Assert.False(built.Standard.IsMissing("fapo", null));
            Assert.Equal("400", built.Standard.Value("Hpal", "ugol", 0, data.ValueSet));
            Assert.True(built.Standard.IsMissing("Eevi", GameData.Of(2, 0).ExistenceSet));
            Assert.False(built.Standard.IsMissing("spur", GameData.Of(2, 0).ExistenceSet));
        }

        [Fact]
        public void standard_fields_and_ids_by_kind()
        {
            var standard = built.Standard;

            Assert.Equal(standard.Fields("unit"), standard.Fields("hero"));
            Assert.Contains(new StandardData.ObjectField("Htb1", "float", true, 1, false, "unreal"), standard.Fields("ability"));
            var attack = standard.Fields("unit").Single(field => field.Id == "ua1t");
            Assert.Equal("choice", attack.Type);
            Assert.Equal("AttackType", attack.Choice!.Name);
            Assert.False(attack.Choice.Flags);
            Assert.Equal(new[] { "unknown", "normal", "pierce", "siege", "spells", "chaos", "magic", "hero" }, attack.Choice.Members.Select(member => member.Text));
            var targets = standard.Fields("ability").Single(field => field.Id == "atar").Choice!;
            Assert.True(targets.Flags);
            Assert.Equal(new[] { "alive", "aliv" }, targets.Members.Where(member => member.Number == 2).Select(member => new[] { member.Text }.Concat(member.Alternates)).Single());
            Assert.Contains("Hpal", standard.Ids("hero"));
            Assert.Contains("AHtb", standard.Ids("ability"));
            Assert.DoesNotContain("AHtb", standard.Ids("unit"));
        }

        [Theory]
        [InlineData("unit", "ua1t", "magic", 6)]
        [InlineData("unit", "ua1t", "MAGIC", 6)]
        [InlineData("unit", "ua1t", " hero ", 7)]
        [InlineData("unit", "ua1t", "3", 3)]
        [InlineData("unit", "ua1t", "nonsense", 0)]
        [InlineData("unit", "ua1t", null, 0)]
        [InlineData("unit", "ua1g", "air,enemies", 1 | 32)]
        [InlineData("unit", "ua1g", "aliv", 2)]
        [InlineData("unit", "ua1g", "air, alli ,ally", 1 | 4)]
        [InlineData("unit", "ua1g", "0", 0)]
        [InlineData("unit", "ua1g", "", 0)]
        [InlineData("unit", "ua1g", "air,nothing", 1)]
        [InlineData("unit", "utyp", "undead,Mechanical", 2 | 8)]
        [InlineData("unit", "udea", "3", 3)]
        [InlineData("ability", "Eme2", "5", 5)]
        public void a_standard_choice_reads_as_the_net_side_reads_it(string kind, string field, string? text, int expected)
        {
            // The same texts as ChoicesTests in Moonwell.Objects.Tests: the Lua's numbers are the ones .NET reads.
            var choice = built.Standard.Fields(kind).Single(one => one.Id == field).Choice!;

            Assert.Equal(expected, choice.Number(text));
        }

        [Fact]
        public void all_standard_ids_span_the_kinds()
        {
            var all = built.Standard.AllIds();

            Assert.Contains("Hpal", all);
            Assert.Contains("AHtb", all);
            Assert.Contains("hfoo", all);
            Assert.Empty(StandardData.Empty.AllIds());
        }

        [Fact]
        public void standard_objects_missing_from_a_balance_set()
        {
            var standard = built.Standard;

            // Illidan is in the base tables and custom_v1, not in custom_v0 or melee_v0; the Spur of Shadows is in custom_v0 and melee_v0, not in the base tables or custom_v1.
            Assert.False(standard.IsMissing("Eevi", null));
            Assert.True(standard.IsMissing("Eevi", "custom_v0"));
            Assert.True(standard.IsMissing("Eevi", "melee_v0"));
            Assert.False(standard.IsMissing("Eevi", "custom_v1"));
            Assert.True(standard.IsMissing("spur", null));
            Assert.True(standard.IsMissing("spur", "custom_v1"));
            Assert.False(standard.IsMissing("spur", "melee_v0"));
            Assert.False(standard.IsMissing("Hpal", "melee_v0"));
            Assert.False(standard.IsMissing("nope", "melee_v0"));
            Assert.False(StandardData.Empty.IsMissing("Eevi", "custom_v0"));
        }

        [Fact]
        public void setter_outside_ctor()
        {
            AssertFails(
                "SetOutsideCtor",
                "SetOutsideCtor.Reset.Heal sets HitPoints; declared fields are set only in the constructor");
        }

        [Fact]
        public void two_replaces()
        {
            AssertFails("TwoReplaces", "TwoReplaces.A and TwoReplaces.B both replace Moonwell.Objects.Human.Units.Footman");
        }

        [Fact]
        public void replaces_modder()
        {
            AssertFails("ReplacesModder", "ReplacesModder.Veteran replaces ReplacesModder.Captain, which is not a standard object");
        }

        [Fact]
        public void list_add_outside_ctor()
        {
            AssertFails(
                "AddOutsideCtor",
                "AddOutsideCtor.Grow.Learn changes Abilities; declared fields are set only in the constructor");
        }

        [Fact]
        public void list_set_outside_ctor()
        {
            AssertFails(
                "SetListOutsideCtor",
                "SetListOutsideCtor.Grow.Learn changes Abilities; declared fields are set only in the constructor");
        }

        [Fact]
        public void list_replace_outside_ctor()
        {
            AssertFails(
                "ReplaceOutsideCtor",
                "ReplaceOutsideCtor.Grow.Learn changes Abilities; declared fields are set only in the constructor");
        }

        [Fact]
        public void set_by_id_outside_ctor()
        {
            AssertFails(
                "SetByIdOutsideCtor",
                "SetByIdOutsideCtor.Grow.Retune changes uhpm; declared fields are set only in the constructor");
        }

        [Fact]
        public void set_of_another_abilitys_field_names_line()
        {
            AssertFails(
                "OtherAbilityField",
                "Objects.cs:13: OtherAbilityField.Bolt's constructor threw: StormBolt has no field Hhb1, which other abilities have (Parameter 'field')");
        }

        [Fact]
        public void formula_that_throws_names_line()
        {
            AssertFails("FormulaThrows", "Objects.cs:13: FormulaThrows.Bolt's constructor threw: no level 1");
        }

        [Fact]
        public void formula_that_calls_the_game_names_line()
        {
            AssertFails(
                "FormulaCallsGame",
                "Objects.cs:13: FormulaCallsGame.Bolt calls the game while it is declared; game work starts from a reaction or the game's start");
        }

        [Fact]
        public void game_call_names_line()
        {
            AssertFails(
                "GameCall",
                "Objects.cs:13: GameCall.Lucky calls the game while it is declared; game work starts from a reaction or the game's start");
        }

        private static DeclaredObject Find(IReadOnlyList<DeclaredObject> declared, string className)
        {
            return Assert.Single(declared, one => one.ClassName == className);
        }

        private int BaseInt(string standardId, string field)
        {
            return int.Parse(built.Standard.Value(standardId, field, 0, null)!);
        }

        private void AssertFails(string fixture, string message)
        {
            var project = MapProject.Find(Path.Combine(TestFolders.Fixture("ObjectErrors"), fixture));
            var assembly = DotnetBuild.Run(project, false, TextWriter.Null);

            var error = Assert.Throws<BuildException>(() => Declarations.Read(assembly, project, built.Standard, null));

            Assert.Equal(message, error.Message);
        }

        /// <summary>The Objects fixture built once, with its standard data.</summary>
        public sealed class BuiltObjects
        {
            private readonly MapProject project;
            private readonly string assembly;

            public BuiltObjects()
            {
                project = MapProject.Find(TestFolders.Fixture("Objects"));
                assembly = DotnetBuild.Run(project, false, TextWriter.Null);
                Standard = StandardData.Read(Path.Combine(Path.GetDirectoryName(assembly)!, "Moonwell.Objects.dll"));
            }

            public StandardData Standard { get; }

            public DeclarationSet Read(string? set)
            {
                return Declarations.Read(assembly, project, Standard, set);
            }
        }
    }
}
