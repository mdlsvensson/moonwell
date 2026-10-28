namespace Moonwell.Engine
{
    /// <summary>A unit's push, as floats: its velocity, its lift, its slowing and who pushed it. One per pushed unit, in the pushes' list; it ends when the push does and the list drops it after the step.</summary>
    internal sealed class Pushed
    {
        /// <summary>The unit pushed.</summary>
        internal readonly Unit unit;

        /// <summary>The velocity along x, in game units a second.</summary>
        internal float vx;

        /// <summary>The velocity along y.</summary>
        internal float vy;

        /// <summary>The velocity up; read only while lifted.</summary>
        internal float vz;

        /// <summary>Whether the unit is lifted: off its height, or with a velocity up or down.</summary>
        internal bool lifted;

        /// <summary>The unit's fly height while lifted.</summary>
        internal float height;

        /// <summary>The height a lift ends at: the ground (0) for a unit that does not fly, its type's fly height for one that flies.</summary>
        internal float home;

        /// <summary>Whether the unit flies: it is kept on flyable ground and drifts back to its height instead of falling.</summary>
        internal readonly bool flyer;

        /// <summary>The unit's collision size: how far ahead of it the ground is checked.</summary>
        internal readonly float size;

        /// <summary>The unit's Body when the push began or was added to; null for none.</summary>
        internal Body? body;

        /// <summary>The latest Knockback's slowing, in game units a second each second; used while knockLeft is more than 0.</summary>
        internal float knockSlowing;

        /// <summary>The seconds of the latest Knockback still to run; its slowing holds until then, Body.Friction after.</summary>
        internal float knockLeft;

        /// <summary>The unit behind the latest push that had one; null for none.</summary>
        internal Unit? pusher;

        /// <summary>Whether the push has ended; the list drops it after the step.</summary>
        internal bool ended;

        /// <summary>Starts the push of a unit, with no velocity yet.</summary>
        internal Pushed(Unit unit, bool flyer, float size)
        {
            this.unit = unit;
            this.flyer = flyer;
            this.size = size;
        }
    }
}
