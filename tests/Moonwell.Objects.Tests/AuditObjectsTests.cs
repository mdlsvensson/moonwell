using System.Reflection;

namespace Moonwell.Objects.Tests
{
    /// <summary>The generated standard classes after the full audit's object fixes (OB1 to OB4 and the smaller ones).</summary>
    public class AuditObjectsTests
    {
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
    }
}
