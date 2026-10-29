namespace Moonwell.Testing
{
    /// <summary>A unit whose OnDeath makes a timer with After, which is the unit's, and one through a helper given the game.</summary>
    public sealed class TimedDeath : Unit
    {
        /// <summary>The timer OnDeath made with After.</summary>
        public Timer? Made;

        /// <summary>The timer the helper made for the game.</summary>
        public Timer? Smoked;

        /// <summary>How many times the timer OnDeath made has run.</summary>
        public int Ran;

        /// <summary>Makes a timer in 30 seconds with After, and one through the helper for the game.</summary>
        public override void OnDeath(Unit? killerUnit)
        {
            Made = After(30.0f, () => { Ran++; });
            Smoked = ScopeCases.Smoke(Game.Instance);
        }
    }
}
