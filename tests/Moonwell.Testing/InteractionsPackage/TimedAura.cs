namespace Moonwell.Testing
{
    /// <summary>An aura whose OnMeet makes a timer with After, the interaction's, and one with the pair's After; its OnPart makes one with After.</summary>
    public sealed class TimedAura : Interaction<Unit, Unit>
    {
        /// <summary>The timer OnMeet made with After.</summary>
        public Timer? Mine;

        /// <summary>The timer OnMeet made with the pair's After.</summary>
        public Timer? Paired;

        /// <summary>The timer OnPart made with After.</summary>
        public Timer? Parted;

        /// <summary>How many times the timer OnPart made has run.</summary>
        public int PartRan;

        /// <summary>Makes an aura of range 300 with a tick of 0.5 seconds.</summary>
        public TimedAura()
        {
            Range = 300.0f;
            Tick = 0.5f;
        }

        /// <summary>Makes a timer with After and one with the pair's After, each in 100 seconds.</summary>
        public override void OnMeet(Pair pair)
        {
            Mine = After(100.0f, () => { });
            Paired = pair.After(100.0f, () => { });
        }

        /// <summary>Makes a timer in 1 second with After.</summary>
        public override void OnPart(Pair pair)
        {
            Parted = After(1.0f, () => { PartRan++; });
        }
    }
}
