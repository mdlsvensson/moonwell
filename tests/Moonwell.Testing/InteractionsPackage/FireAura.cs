using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>An interaction the interactions cases start: held by a unit, it pairs with units. It writes each reaction to the test log as "meet b", "tick b" or "part b", with the name Tag gives the other, notes the cycle's step at each OnTick, and runs the actions a case gives it.</summary>
    public sealed class FireAura : Interaction<Unit, Unit>
    {
        /// <summary>The name the log gives a unit; "?" unless a case says.</summary>
        public Func<Unit, string> Tag = unit => "?";

        /// <summary>What Accepts answers; null accepts every unit.</summary>
        public Func<Unit, bool>? Accepting;

        /// <summary>How many times Accepts ran.</summary>
        public int Asked;

        /// <summary>The current owner when Accepts last ran.</summary>
        public Scope? AskedAs;

        /// <summary>The cycle's step at each OnTick, in turn.</summary>
        public List<int> TickSteps = new List<int>();

        /// <summary>An action OnMeet runs after it has logged; null for none.</summary>
        public Action<Pair>? Meeting;

        /// <summary>An action OnTick runs after it has logged; null for none.</summary>
        public Action<Pair>? Ticking;

        /// <summary>An action OnPart runs after it has logged; null for none.</summary>
        public Action<Pair>? Parting;

        /// <summary>The other whose OnTick throws an InvalidOperationException after it has logged; null for none.</summary>
        public Unit? Throws;

        /// <summary>Makes an aura of a range, 300 unless given, that ticks every tick seconds, half a second unless given.</summary>
        public FireAura(float range = 300.0f, float tick = 0.5f)
        {
            Range = range;
            Tick = tick;
        }

        /// <summary>Counts the ask and notes the current owner, then answers what Accepting says.</summary>
        public override bool Accepts(Unit other)
        {
            Asked++;
            AskedAs = Scope.Current;
            return Accepting == null || Accepting(other);
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

        /// <summary>Logs "tick" and notes the step, then runs the action, then throws for the other it was told to.</summary>
        public override void OnTick(Pair pair)
        {
            TestLog.Lines.Add("tick " + Tag(pair.Other));
            TickSteps.Add(Tracking.StepNow);
            if (Ticking != null)
            {
                Ticking(pair);
            }

            if (Throws != null && pair.Other == Throws)
            {
                throw new InvalidOperationException("boom");
            }
        }

        /// <summary>Logs "part", then runs the action.</summary>
        public override void OnPart(Pair pair)
        {
            TestLog.Lines.Add("part " + Tag(pair.Other));
            if (Parting != null)
            {
                Parting(pair);
            }
        }
    }
}
