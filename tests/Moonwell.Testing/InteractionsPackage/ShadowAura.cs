namespace Moonwell.Testing
{
    /// <summary>An interaction with fields of its own named as the package's fields of Interaction and Interaction&lt;THolder, TOther&gt;. In Lua a class's fields share one table with its base's, so each of these must stay apart from the base's field of the same name.</summary>
    public sealed class ShadowAura : Interaction<Unit, Unit>
    {
        // A map cannot see the package's internal fields, so these hide nothing there; the tests can see them, and C# warns.
#pragma warning disable CS0108

        /// <summary>Its own paused, true; the interaction is not paused.</summary>
        private bool paused = true;

        /// <summary>Its own range, 1; the interaction's Range is 300.</summary>
        private float range = 1.0f;

        /// <summary>Its own tick, 2; the interaction's Tick is half a second.</summary>
        private float tick = 2.0f;

        /// <summary>Its own started, true; the interaction has not been started when it is made.</summary>
        private bool started = true;

        /// <summary>Its own due, 7; the interaction has no place in the cycle when it is made.</summary>
        private int due = 7;

        /// <summary>Its own holder, none; the interaction's Holder is the unit it is started on.</summary>
        private Unit? holder = null;

#pragma warning restore CS0108

        /// <summary>Makes an aura of range 300 that checks every half second.</summary>
        public ShadowAura()
        {
            Range = 300.0f;
            Tick = 0.5f;
        }

        /// <summary>Its own paused.</summary>
        public bool OwnPaused
        {
            get { return paused; }
        }

        /// <summary>Its own range.</summary>
        public float OwnRange
        {
            get { return range; }
        }

        /// <summary>Its own tick.</summary>
        public float OwnTick
        {
            get { return tick; }
        }

        /// <summary>Its own started.</summary>
        public bool OwnStarted
        {
            get { return started; }
        }

        /// <summary>Its own due.</summary>
        public int OwnDue
        {
            get { return due; }
        }

        /// <summary>Its own holder.</summary>
        public Unit? OwnHolder
        {
            get { return holder; }
        }

        /// <summary>Logs "meet".</summary>
        public override void OnMeet(Pair pair)
        {
            TestLog.Lines.Add("meet");
        }
    }
}
