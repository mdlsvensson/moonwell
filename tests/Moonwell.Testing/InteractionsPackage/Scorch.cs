namespace Moonwell.Testing
{
    /// <summary>An interaction the interactions cases start on a unit: it pairs with destructables within 300 and checks at every step. It writes "meet", "tick" and "part" to the test log.</summary>
    public sealed class Scorch : Interaction<Unit, Destructable>
    {
        /// <summary>Makes a scorch of range 300 that checks at every step.</summary>
        public Scorch()
        {
            Range = 300.0f;
        }

        /// <summary>Logs "meet".</summary>
        public override void OnMeet(Pair pair)
        {
            TestLog.Lines.Add("meet");
        }

        /// <summary>Logs "tick".</summary>
        public override void OnTick(Pair pair)
        {
            TestLog.Lines.Add("tick");
        }

        /// <summary>Logs "part".</summary>
        public override void OnPart(Pair pair)
        {
            TestLog.Lines.Add("part");
        }
    }
}
