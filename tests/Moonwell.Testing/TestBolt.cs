namespace Moonwell.Testing
{
    /// <summary>An ability that logs its cast, keeps the cast and the owner it saw, and starts a timer the ability owns and one the cast owns.</summary>
    public class TestBolt : Ability
    {
        /// <summary>The cast the last OnCast was given; null when none.</summary>
        public Cast? Given;

        /// <summary>How many times the timer started by the last cast with no owner given has run.</summary>
        public int Ticks;

        /// <summary>How many times the timer started by the last cast with the cast as its owner has run.</summary>
        public int ChannelTicks;

        /// <summary>Logs the cast, keeps it, and starts two timers every half second that count their ticks: one with no owner given, which the ability owns, and one the cast owns, as work that stops with a channel.</summary>
        public override void OnCast(Cast cast)
        {
            TestLog.Lines.Add("bolt cast");
            Given = cast;
            Every(0.5f, () => { Ticks++; });
            cast.Every(0.5f, () => { ChannelTicks++; });
        }
    }
}
