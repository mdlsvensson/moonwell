using Moonwell.Objects.Abilities.Human;
using Moonwell.Objects.Units.Human;

namespace MoonwellMap
{
    /// <summary>A Paladin of the map's own: Smite in place of Holy Light, and Fury from the start.</summary>
    public sealed class Champion : Paladin
    {
        public Champion()
        {
            Name = "Champion";
            HitPoints = HitPoints + 200;
            HeroAbilities.Remove<HolyLight>();
            HeroAbilities.Add<Smite>();
            Components.Add(new Fury());
        }
    }
}
