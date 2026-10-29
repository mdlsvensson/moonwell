namespace Moonwell.Testing
{
    /// <summary>A scope whose OnEnd makes a timer with After, in itself while it ends.</summary>
    public sealed class EndingTimerScope : Scope
    {
        /// <summary>The timer OnEnd made.</summary>
        public Timer? Made;

        /// <summary>Makes a timer in 1 second with After.</summary>
        protected override void OnEnd()
        {
            Made = After(1.0f, () => { });
        }
    }
}
