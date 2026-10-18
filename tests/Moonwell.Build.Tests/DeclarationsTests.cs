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
        public void reads_arthas()
        {
            var arthas = Find(built.Read(null), "Objects.Arthas");

            Assert.Equal("hero", arthas.Kind);
            Assert.Equal("Hpal", arthas.StandardRoot);
            Assert.False(arthas.Replaces);
            Assert.Null(arthas.PinnedId);
            Assert.Equal(new[] { "uabi", "ugol", "uhab", "uhpm", "unam" }, arthas.Changed.Keys);
            Assert.Equal("Arthas", arthas.Changed["unam"]);
            Assert.Equal(2 * BaseInt("Hpal", "uhpm"), arthas.Changed["uhpm"]);
            Assert.Equal(2 * BaseInt("Hpal", "ugol"), arthas.Changed["ugol"]);
            Assert.Equal(
                new[] { "Moonwell.Objects.Abilities.Other.Inventory", "Objects.Firebolt" },
                Assert.IsAssignableFrom<IReadOnlyList<string>>(arthas.Changed["uabi"]));
            // The Paladin's hero abilities without the one Arthas removed; it stands in the standard list before.
            var heroAbilities = Assert.IsAssignableFrom<IReadOnlyList<string>>(arthas.Changed["uhab"]);
            Assert.Equal(
                new[]
                {
                    "Moonwell.Objects.Abilities.Human.HolyLight",
                    "Moonwell.Objects.Abilities.Human.DivineShield",
                    "Moonwell.Objects.Abilities.Human.Resurrection",
                },
                heroAbilities);
            Assert.Equal(
                new[]
                {
                    "Moonwell.Objects.Abilities.Human.DivineShield",
                    "Moonwell.Objects.Abilities.Human.HolyLight",
                    "Moonwell.Objects.Abilities.Human.Resurrection",
                    "Moonwell.Objects.Abilities.Other.Inventory",
                    "Objects.Firebolt",
                },
                arthas.References);
        }

        [Fact]
        public void contains_and_add_see_the_standard_list()
        {
            var duelist = Find(built.Read(null), "Objects.Duelist");

            // Contains<Defend> was true in the constructor; Footman's list is Defend and Aihn, then Firebolt (added in a lambda).
            Assert.Equal("Defender", duelist.Changed["unam"]);
            var abilities = Assert.IsAssignableFrom<IReadOnlyList<string>>(duelist.Changed["uabi"]);
            Assert.Equal(3, abilities.Count);
            Assert.Equal("Moonwell.Objects.Abilities.Human.Defend", abilities[0]);
            Assert.Equal("Objects.Firebolt", abilities[2]);
        }

        [Fact]
        public void a_reference_and_a_bool_are_read()
        {
            var duelist = Find(built.Read(null), "Objects.Duelist");

            Assert.Equal("Objects.Firebolt", duelist.Changed["udaa"]);
            Assert.Equal(true, duelist.Changed["ubdg"]);
            Assert.Contains("Objects.Firebolt", duelist.References);
        }

        [Fact]
        public void a_reference_of_each_level()
        {
            var elemental = Find(built.Read(null), "Objects.Elemental");

            Assert.Equal("ability", elemental.Kind);
            Assert.Equal("AHwe", elemental.StandardRoot);
            Assert.Equal(new[] { "Hwe1:1", "Hwe1:2", "Hwe1:3" }, elemental.Changed.Keys);
            Assert.All(elemental.Changed.Values, value => Assert.Equal("Objects.Arthas", value));
            Assert.Equal(new[] { "Objects.Arthas" }, elemental.References);
        }

        [Fact]
        public void a_list_of_each_level_is_one_list()
        {
            var portal = Find(built.Read(null), "Objects.Portal");

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
            var tuned = Find(built.Read(null), "Objects.Tuned");

            Assert.Equal(100f, tuned.Changed["Htb1:1"]);
            Assert.Equal(250f, tuned.Changed["Htb1:2"]);
            Assert.Equal(300f, tuned.Changed["Htb1:3"]);
        }

        [Fact]
        public void unset_field_reads_chosen_set()
        {
            // Hpal's gold cost is 400 in the base tables and 500 in melee_v0.
            Assert.Equal("500", built.Standard.Value("Hpal", "ugol", 0, "melee_v0"));

            var arthas = Find(built.Read("melee_v0"), "Objects.Arthas");

            Assert.Equal(1000, arthas.Changed["ugol"]);
            Assert.Equal(200, arthas.Changed["uhpm"]);
        }

        [Fact]
        public void chain_merges()
        {
            var arthas = Find(built.Read(null), "Objects.Arthas");
            var dark = Find(built.Read(null), "Objects.DarkArthas");

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
            var firebolt = Find(built.Read(null), "Objects.Firebolt");

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
            var declared = built.Read(null);

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
            var declared = built.Read(null);

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
            var declared = Find(built.Read(null), className);

            Assert.Equal(kind, declared.Kind);
            Assert.Equal(root, declared.StandardRoot);
            Assert.Equal(value, declared.Changed[field]);
        }

        [Fact]
        public void ordered_by_class_name()
        {
            var names = built.Read(null).Select(one => one.ClassName).ToList();

            Assert.Equal(
                new[]
                {
                    "Objects.Arthas", "Objects.Burn", "Objects.Claws", "Objects.DarkArthas", "Objects.Duelist",
                    "Objects.Elemental", "Objects.Firebolt", "Objects.Forge", "Objects.Gate", "Objects.Portal",
                    "Objects.Rock", "Objects.StrongFootman", "Objects.Tuned",
                },
                names);
        }

        [Fact]
        public void map_without_objects_declares_none()
        {
            var project = MapProject.Find(TestFolders.Fixture("CallsGame"));
            var assembly = DotnetBuild.Run(project, false, TextWriter.Null);

            Assert.Empty(Declarations.Read(assembly, project, built.Standard, null));
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

        [Fact]
        public void standard_fields_and_ids_by_kind()
        {
            var standard = built.Standard;

            Assert.Equal(standard.Fields("unit"), standard.Fields("hero"));
            Assert.Contains(new StandardData.ObjectField("Htb1", "float", true, 1, false, "unreal"), standard.Fields("ability"));
            Assert.Contains("Hpal", standard.Ids("hero"));
            Assert.Contains("AHtb", standard.Ids("ability"));
            Assert.DoesNotContain("AHtb", standard.Ids("unit"));
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
            AssertFails("TwoReplaces", "TwoReplaces.A and TwoReplaces.B both replace Moonwell.Objects.Units.Human.Footman");
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
        public void set_by_id_outside_ctor()
        {
            AssertFails(
                "SetByIdOutsideCtor",
                "SetByIdOutsideCtor.Grow.Retune changes uhpm; declared fields are set only in the constructor");
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

            public IReadOnlyList<DeclaredObject> Read(string? set)
            {
                return Declarations.Read(assembly, project, Standard, set);
            }
        }
    }
}
