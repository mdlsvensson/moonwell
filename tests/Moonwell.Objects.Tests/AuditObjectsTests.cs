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
