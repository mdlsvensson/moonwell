using static Moonwell.Objects.Human.Units;

namespace Sample
{
    /// <summary>The sample map's one object: a Paladin with a name of his own.</summary>
    [Moonwell.Objects.ObjectId("H001")]
    public sealed class Arthas : Paladin
    {
        public Arthas()
        {
            Name = "Arthas";
        }
    }
}
