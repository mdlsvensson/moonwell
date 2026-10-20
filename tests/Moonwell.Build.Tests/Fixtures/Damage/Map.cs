using Moonwell;

namespace DamageFixture
{
    /// <summary>A map that uses the damage package: a footman of its own that takes half of every hit, and a hit dealt at the start.</summary>
    public sealed class DamageMap : Map
    {
        public DamageMap()
        {
            Name = "Damage";
            Components.Add(new Opening());
        }
    }
}
