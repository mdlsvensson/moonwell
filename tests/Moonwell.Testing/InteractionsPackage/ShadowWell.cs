namespace Moonwell.Testing
{
    /// <summary>A gizmo with fields of its own named as the package's fields of Gizmo. In Lua a class's fields share one table with its base's, so each of these must stay apart from the base's field of the same name.</summary>
    public sealed class ShadowWell : Gizmo
    {
        // A map cannot see the package's internal fields, so these hide nothing there; the tests can see them, and C# warns.
#pragma warning disable CS0108

        /// <summary>Its own height, 5; the gizmo's Z is 0 until set.</summary>
        private float z = 5.0f;

        /// <summary>Its own player, none; the gizmo plays for Neutral Passive.</summary>
        private Player? player = null;

        /// <summary>Its own tick, 3; the gizmo's Tick is a step.</summary>
        private float tick = 3.0f;

        /// <summary>Its own due, 9; the gizmo's place in the cycle is the package's.</summary>
        private int due = 9;

#pragma warning restore CS0108

        /// <summary>How many times OnTick ran.</summary>
        public int Ticks;

        /// <summary>Makes a well that ticks at every step.</summary>
        public ShadowWell()
        {
            Tick = Interactions.Step;
        }

        /// <summary>Its own height.</summary>
        public float OwnZ
        {
            get { return z; }
        }

        /// <summary>Its own player.</summary>
        public Player? OwnPlayer
        {
            get { return player; }
        }

        /// <summary>Its own tick.</summary>
        public float OwnTick
        {
            get { return tick; }
        }

        /// <summary>Its own due.</summary>
        public int OwnDue
        {
            get { return due; }
        }

        /// <summary>Counts the tick.</summary>
        public override void OnTick()
        {
            Ticks++;
        }
    }
}
