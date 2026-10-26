using System;

namespace Moonwell.Testing
{
    /// <summary>An interaction the interactions cases start on an item or a destructable: held by any thing with a place, it pairs with units within 300 and checks at every step. It writes "meet b" and "part b" to the test log, with the name Tag gives the other.</summary>
    public sealed class Shine : Interaction<Actor, Unit>
    {
        /// <summary>The name the log gives a unit; "?" unless a case says.</summary>
        public Func<Unit, string> Tag = unit => "?";

        /// <summary>Makes a shine of range 300 that checks at every step.</summary>
        public Shine()
        {
            Range = 300.0f;
        }

        /// <summary>Logs "meet".</summary>
        public override void OnMeet(Pair pair)
        {
            TestLog.Lines.Add("meet " + Tag(pair.Other));
        }

        /// <summary>Logs "part".</summary>
        public override void OnPart(Pair pair)
        {
            TestLog.Lines.Add("part " + Tag(pair.Other));
        }
    }
}
