using Moonwell.Engine;

namespace Moonwell
{
    /// <summary>
    /// Firing missiles from a unit: <c>caster.Fire(new Firebolt(), target)</c>. The missile starts at the unit, its
    /// LaunchOffset ahead and its LaunchHeight up, with the unit as its Source and the unit's player as its own, and flies
    /// at its Speed. It belongs to the owner given, else to the game, so a missile fired in a spell's reaction outlives the
    /// cast. With no Arc it flies on past the mark until something stops it; with an Arc it lands on the mark.
    /// </summary>
    public static class FireExtensions
    {
        /// <summary>Fires a missile at a unit where it is now: at its middle, half its height above the ground and its fly height, and returns the missile. Each Homing its class lists with no target is given the unit, before OnCreated runs.</summary>
        public static T Fire<T>(this Unit source, T missile, Unit target, Owner? owner = null)
            where T : Missile
        {
            var starting = missile.StartingComponents();
            for (var i = 0; i < starting.Count; i++)
            {
                var homing = starting[i] as Homing;
                if (homing != null && homing.Target == null)
                {
                    homing.Target = target;
                }
            }

            var above = target.FlyHeight + Shapes.UnitHeight(target) * 0.5f;
            Flight.Launch(source, missile, target.X, target.Y, above, false, owner);
            return missile;
        }

        /// <summary>Fires a missile at a point, and returns it: with no Arc at the height above the ground there that it left at, so on flat ground it flies level; with an Arc it lands on the ground there, its middle aimed its radius above the ground so its bottom meets the ground at the point.</summary>
        public static T Fire<T>(this Unit source, T missile, float x, float y, Owner? owner = null)
            where T : Missile
        {
            Flight.Launch(source, missile, x, y, 0.0f, true, owner);
            return missile;
        }

        /// <summary>Fires a missile at a point and height, and returns it; with an Arc it lands there.</summary>
        public static T Fire<T>(this Unit source, T missile, float x, float y, float z, Owner? owner = null)
            where T : Missile
        {
            Flight.Launch(source, missile, x, y, z - Ground.Height(x, y), false, owner);
            return missile;
        }
    }
}
