namespace Moonwell.Testing
{
    /// <summary>An ability that logs its cast, keeps the cast and the owner it saw, and starts a timer the cast owns.</summary>
    public class TestBolt : Ability
    {
        /// <summary>The cast the last OnCast was given; null when none.</summary>
        public Cast? Given;

        /// <summary>The owner that was current when the last OnCast ran.</summary>
        public Owner? Seen;

        /// <summary>How many times the timer started by the last cast has run.</summary>
        public int Ticks;

        /// <summary>Logs the cast, keeps it, and starts a timer every half second that counts its ticks.</summary>
        public override void OnCast(Cast cast)
        {
            TestLog.Lines.Add("bolt cast");
            Given = cast;
            Seen = Owner.Current;
            Timer.Every(0.5f, () => { Ticks++; });
        }
    }
}
