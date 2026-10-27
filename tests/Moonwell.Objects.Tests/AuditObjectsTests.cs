using System.Reflection;

namespace Moonwell.Objects.Tests
{
    /// <summary>The generated standard classes after the full audit's object fixes (OB1 to OB4 and the smaller ones).</summary>
    [Collection(StandardValuesCollection.Name)]
    public class AuditObjectsTests : IDisposable
    {
        public AuditObjectsTests()
        {
            StandardValues.Read = null;
        }

        public void Dispose()
        {
            StandardValues.Read = null;
        }

        private static string IdOf(Type type)
        {
            return type.GetCustomAttribute<StandardAttribute>(false)!.Id;
        }

        [Fact]
        public void in_a_clash_the_plain_name_is_the_object_without_an_editor_suffix()
        {
            // Decision 71: the Tauren Chieftain's Shockwave, not Cairne's; the tavern Alchemist, not his morph form.
            Assert.Equal("AOsh", IdOf(typeof(Orc.Abilities.Shockwave)));
            Assert.Equal("AOs2", IdOf(typeof(Orc.Abilities.ShockwaveAOs2)));
            Assert.Equal("Nalc", IdOf(typeof(Creeps.Units.Alchemist)));
            Assert.Equal("Nal2", IdOf(typeof(Creeps.Units.AlchemistNal2)));
            Assert.Equal("AOre", IdOf(typeof(Orc.Abilities.Reincarnation)));
            Assert.Equal("AHgr", IdOf(typeof(Other.Abilities.Grit)));

            // Neither Tinker has an editor suffix, so the first id keeps the plain name: the Robo-Goblin form.
            Assert.Equal("Nrob", IdOf(typeof(Creeps.Units.Tinker)));
        }

        /// <summary>A transport's hold of a map's own with room for six.</summary>
        public class SixSeatHold : Other.Abilities.CargoHold
        {
            public SixSeatHold()
            {
                Set("Car1", 1, 6);
            }
        }

        /// <summary>A Hydra of a map's own that sets nothing.</summary>
        public class PlainHydra : Creeps.Units.Hydra
        {
        }

        [Fact]
        public void a_standard_list_entry_after_a_space_is_its_class()
        {
            // The Hydra's abilities are "Aspo, Aspt": with no build the constructor's list, with one the build's text.
            Assert.True(new PlainHydra().Abilities.Contains<Creeps.Abilities.SpawnHydraHatchling>());

            StandardValues.Read = (id, field, level) => id == "nhyd" && field == "uabi" && level == 0 ? "Aspo, Aspt,," : null;
            var abilities = new PlainHydra().Abilities;
            Assert.True(abilities.Contains<Creeps.Abilities.SpawnHydraHatchling>());
            Assert.Equal(2, abilities.Count);
        }

        /// <summary>A Storm Bolt of a map's own with six levels and nothing else set.</summary>
        public class SixBolt : Human.Abilities.StormBolt
        {
            public SixBolt()
            {
                LevelCount = 6;
            }
        }

        [Fact]
        public void a_level_past_the_standard_level_count_reads_the_games_value()
        {
            // The game gives a Storm Bolt of six levels 325 damage, 9 cooldown and 75 mana at levels 4 to 6 (game-facts, 3.0.1).
            StandardValues.Read = (id, field, level) => ValueOf(id, level == 0 ? field : field + ":" + level);
            var bolt = new SixBolt();

            Assert.Equal(310f, bolt.Damage[3]);
            Assert.Equal(325f, bolt.Damage[4]);
            Assert.Equal(325f, bolt.Damage[6]);
            Assert.Equal(75, bolt.ManaCost[5]);
            Assert.Equal(9f, bolt.Cooldown[6]);
        }

        // A standard value of the generated values, as the build reads it; null for none.
        private static string? ValueOf(string id, string key)
        {
            using var stream = typeof(StandardAttribute).Assembly.GetManifestResourceStream("Moonwell.Objects.Values.ability.json")!;
            using var document = System.Text.Json.JsonDocument.Parse(stream);
            return document.RootElement.GetProperty("objects").TryGetProperty(id, out var one)
                && one.GetProperty("values").TryGetProperty(key, out var value) ? value.GetString() : null;
        }

        [Fact]
        public void the_placement_fields_hold_world_editors_names()
        {
            // Decision 70: a human building's preventPlace "unbuildable" is World Editor's "Placement Requires: Buildable".
            Assert.Equal(PathingRequire.Buildable, new Human.Units.Barracks().PlacementRequires);
            Assert.Equal("unbuildable", typeof(PathingRequire).GetField(nameof(PathingRequire.Buildable))!.GetCustomAttribute<GameValueAttribute>()!.Text);
            Assert.Equal("blighted", typeof(PathingPrevent).GetField(nameof(PathingPrevent.Unblighted))!.GetCustomAttribute<GameValueAttribute>()!.Text);
            Assert.Equal(typeof(PathingPrevent), typeof(StandardUnit).GetProperty("PlacementPreventedBy")!.PropertyType);
        }

        [Fact]
        public void a_melee_hero_is_named_by_its_type_and_a_campaign_hero_by_its_one_proper_name()
        {
            Assert.Equal("Haah", IdOf(typeof(Human.Units.HighElfArchmage)));
            Assert.Equal("Nglx", IdOf(typeof(Creeps.Units.Merchant)));
            Assert.Equal("Ecen", IdOf(typeof(Campaign.Units.Cenarius)));
        }

        [Fact]
        public void a_target_type_of_self_is_named_self()
        {
            // worldeditstrings.txt gives the label as "Self", then again as "Shelf"; the first one is the name.
            Assert.Equal(0, (int)ProctargetType.Self);
            Assert.Null(typeof(ProctargetType).GetField("Shelf"));
        }

        private static IEnumerable<string> FieldsOf(Type type)
        {
            return type.GetProperties(BindingFlags.Public | BindingFlags.Instance | BindingFlags.DeclaredOnly)
                .Select(property => property.GetCustomAttribute<FieldAttribute>()?.Id)
                .Where(id => id != null)
                .Select(id => id!);
        }

        [Fact]
        public void an_ability_has_the_fields_the_metadata_lists_by_its_own_id()
        {
            // Cargo Capacity lists Sch2 to Sch5, whose code is Amtc; Command Aura's ACac lists itself, not AOac.
            Assert.Contains("Car1", FieldsOf(typeof(Other.Abilities.CargoHold)));
            Assert.NotNull(new SixSeatHold());
            Assert.Contains("Cac1", FieldsOf(typeof(Creeps.Abilities.CommandAura)));
            Assert.Contains("Eme1", FieldsOf(typeof(Creeps.Abilities.Submerge)));

            // AIls (code Alsh) has Idps on DataA, listed by its id, not Lightning Shield's Lsh1.
            Assert.Contains("Idps", FieldsOf(typeof(Other.Abilities.LightningShield)));
            Assert.DoesNotContain("Lsh1", FieldsOf(typeof(Other.Abilities.LightningShield)));
        }
    }
}
