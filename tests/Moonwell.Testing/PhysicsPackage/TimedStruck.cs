namespace Moonwell.Testing
{
    /// <summary>A unit whose OnStruck makes a timer with After, which is the unit's.</summary>
    public sealed class TimedStruck : Unit, IMissileTarget
    {
        /// <summary>The timer OnStruck last made.</summary>
        public Timer? Made;

        /// <summary>How many times a timer OnStruck made has run.</summary>
        public int Ran;

        /// <summary>Makes a timer in 1 second with After.</summary>
        public void OnStruck(Impact impact)
        {
            Made = After(1.0f, () => { Ran++; });
        }
    }
}
