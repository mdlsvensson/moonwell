namespace Moonwell.Testing
{
    /// <summary>A buff of 1 second whose OnExpire makes a timer with After, in itself as it ends.</summary>
    public sealed class ExpiringTimerBuff : Buff
    {
        /// <summary>The timer OnExpire made.</summary>
        public Timer? Made;

        /// <summary>Makes a buff of 1 second.</summary>
        public ExpiringTimerBuff()
        {
            Duration = 1.0f;
        }

        /// <summary>Makes a timer in 1 second with After.</summary>
        public override void OnExpire()
        {
            Made = After(1.0f, () => { });
        }
    }
}
