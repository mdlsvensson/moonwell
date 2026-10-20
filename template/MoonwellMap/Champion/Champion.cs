using static Moonwell.Objects.Human.Abilities;
using static Moonwell.Objects.Human.Units;

namespace MoonwellMap
{
    /// <summary>A Paladin of the map's own: Smite in place of Holy Light, and Fury from the start.</summary>
    [Moonwell.Objects.ObjectId("H001")]
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
