using Moonwell.Engine;

namespace Moonwell
{
    /// <summary>
    /// Pushing units: <c>target.Knockback(caster.AngleTo(target), 300f, 0.5f)</c>. A push moves the unit each step with
    /// SetUnitX/Y, which keeps its order, so its own walking carries on and adds to it; it is kept on ground it could
    /// stand on, sliding along what blocks it or stopping (OnSlammed). A push up lifts it: a unit that does not fly
    /// falls back under Physics.Gravity (OnLanded), a flying one drifts back to its height. It slows by its Body's
    /// Friction a second unless a Knockback set its own, and ends when it stops, by StopPush, or when the unit dies,
    /// leaves the game, is hidden or is loaded. A structure, or a unit whose Body is Immovable, is never pushed.
    /// </summary>
    public static class PushExtensions
    {
        /// <summary>Adds a velocity to the unit's push, in game units a second: along x, along y, and up. OnPushed runs, with no pusher.</summary>
        public static void Push(this Unit unit, float vx, float vy, float vz = 0.0f)
        {
            Pushes.Push(unit, vx, vy, vz, null);
        }

        /// <summary>Knocks the unit a distance along an angle in degrees over a time in seconds, slowing evenly to a stop: a push of 2 * distance / seconds that slows by its speed / seconds a second. It adds to a push already on the unit, and its slowing replaces the push's for its time. A distance of 0 does nothing; a time of 0 or less fails in a development build and does nothing in a release build.</summary>
        public static void Knockback(this Unit unit, float angle, float distance, float seconds)
        {
            Pushes.Knockback(unit, angle, distance, seconds);
        }

        /// <summary>The unit's push as a velocity in game units a second, its own walking not counted; zero when it is not pushed. A vector is a table, made when read.</summary>
        [LocalSafe]
        public static Vector3 PushVelocity(this Unit unit)
        {
            float vx;
            float vy;
            float vz;
            Pushes.Velocity(unit, out vx, out vy, out vz);
            return new Vector3(vx, vy, vz);
        }

        /// <summary>Whether the unit is being pushed: moving, or lifted and not yet back down.</summary>
        [LocalSafe]
        public static bool IsPushed(this Unit unit)
        {
            return Pushes.Find(unit) != null;
        }

        /// <summary>Ends the unit's push where it is, with no reaction; a lifted unit is put back at its height (on the ground for one that does not fly). Nothing when it is not pushed.</summary>
        public static void StopPush(this Unit unit)
        {
            Pushes.Stop(unit);
        }
    }
}
