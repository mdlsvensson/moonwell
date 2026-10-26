using System;

namespace Moonwell.Testing
{
    /// <summary>An interaction the interactions cases start on a gizmo: held by a well, it pairs with units within 300 and checks at every step. It writes each reaction to the test log as FireAura does, and runs the action a case gives OnMeet.</summary>
    public sealed class Healing : Interaction<Well, Unit>
    {
        /// <summary>The name the log gives a unit; "?" unless a case says.</summary>
        public Func<Unit, string> Tag = unit => "?";

        /// <summary>An action OnMeet runs after it has logged; null for none.</summary>
        public Action<Pair>? Meeting;

        /// <summary>Makes a healing of range 300 that checks at every step.</summary>
        public Healing()
        {
            Range = 300.0f;
        }

        /// <summary>Logs "meet", then runs the action.</summary>
        public override void OnMeet(Pair pair)
        {
            TestLog.Lines.Add("meet " + Tag(pair.Other));
            if (Meeting != null)
            {
                Meeting(pair);
            }
        }

        /// <summary>Logs "tick".</summary>
        public override void OnTick(Pair pair)
        {
            TestLog.Lines.Add("tick " + Tag(pair.Other));
        }

        /// <summary>Logs "part".</summary>
        public override void OnPart(Pair pair)
        {
            TestLog.Lines.Add("part " + Tag(pair.Other));
        }
    }
}
