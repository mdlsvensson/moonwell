namespace Moonwell.Testing
{
    /// <summary>An aura whose OnPart makes a timer with the pair's After, in the pair as it ends.</summary>
    public sealed class PartingTimerAura : Interaction<Unit, Unit>
    {
        /// <summary>The timer OnPart made.</summary>
        public Timer? Made;

        /// <summary>Makes an aura of range 300 with a tick of 0.5 seconds.</summary>
        public PartingTimerAura()
        {
            Range = 300.0f;
            Tick = 0.5f;
        }

        /// <summary>Makes a timer in 1 second with the pair's After.</summary>
        public override void OnPart(Pair pair)
        {
            Made = pair.After(1.0f, () => { });
        }
    }
}
