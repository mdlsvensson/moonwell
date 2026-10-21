using Moonwell;
using Moonwell.Objects;

namespace MoonwellMap.Heroes
{
    /// <summary>A Paladin of the map's own: Smite in place of Holy Light, and a kill counter from the start.</summary>
    [ObjectId("H001")]
    public sealed class Champion : Human.Units.Paladin
    {
        public Champion()
        {
            Name = "Champion";
            MaxLife = MaxLife + 200;
            HeroAbilities.Replace<Human.Abilities.HolyLight, Smite>();
            Components.Add(new KillCounter());
        }
    }
}
