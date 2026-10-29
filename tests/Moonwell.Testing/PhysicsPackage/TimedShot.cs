namespace Moonwell.Testing
{
    /// <summary>A missile whose OnImpact makes a timer with After, which is the missile's.</summary>
    public sealed class TimedShot : Missile
    {
        /// <summary>The timer OnImpact last made.</summary>
        public Timer? Made;

        /// <summary>How many times a timer OnImpact made has run.</summary>
        public int Ran;

        /// <summary>Makes a timer in 1 second with After.</summary>
        public override void OnImpact(Impact impact)
        {
            Made = After(1.0f, () => { Ran++; });
        }
    }
}
