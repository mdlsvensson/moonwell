using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>An area that logs its reactions as "entered a", "stays a", "left a" (the label the case gave the unit), and runs the actions a case gives it.</summary>
    public sealed class LogArea : Area
    {
        /// <summary>The reactions so far, in order.</summary>
        public List<string> Log = new List<string>();

        /// <summary>The label of each unit; "?" for one without.</summary>
        public Dictionary<Unit, string> Labels = new Dictionary<Unit, string>();

        /// <summary>What Accepts answers; null accepts every unit.</summary>
        public Func<Unit, bool>? Accept;

        /// <summary>Run in OnUnitEntered after it is logged; null for nothing.</summary>
        public Action<Unit>? WhenEntered;

        /// <summary>Run in OnUnitStays after it is logged; null for nothing.</summary>
        public Action<Unit>? WhenStays;

        /// <summary>Run in OnUnitLeft after it is logged; null for nothing.</summary>
        public Action<Unit>? WhenLeft;

        /// <summary>The seconds Since gave for each OnUnitStays, in order.</summary>
        public List<float> SinceAtStay = new List<float>();

        /// <summary>The current owner seen in the last reaction.</summary>
        public Owner? OwnerSeen;

        /// <summary>How many times Accepts was asked.</summary>
        public int AcceptsAsked;

        /// <summary>Makes an area of a shape that stays every tick seconds (0 for never).</summary>
        public LogArea(AreaShape shape, float tick = 0f)
        {
            Shape = shape;
            Tick = tick;
        }

        /// <summary>Turns asking Accepts again at the place check on or off.</summary>
        public void SetRecheck(bool on)
        {
            RecheckAccepts = on;
        }

        /// <summary>Sets the shape later than the constructor, for a case that checks it cannot be.</summary>
        public void SetShape(AreaShape shape)
        {
            Shape = shape;
        }

        /// <summary>The label of a unit.</summary>
        public string Label(Unit unit)
        {
            return Labels.TryGetValue(unit, out var label) ? label : "?";
        }

        /// <summary>Asks the case's answer.</summary>
        public override bool Accepts(Unit unit)
        {
            AcceptsAsked++;
            return Accept == null || Accept(unit);
        }

        /// <summary>Logs the unit.</summary>
        public override void OnUnitEntered(Unit enteringUnit)
        {
            OwnerSeen = Owner.Current;
            Log.Add("entered " + Label(enteringUnit));
            if (WhenEntered != null)
            {
                WhenEntered(enteringUnit);
            }
        }

        /// <summary>Logs the unit and how long it has been inside.</summary>
        public override void OnUnitStays(Unit stayingUnit)
        {
            OwnerSeen = Owner.Current;
            Log.Add("stays " + Label(stayingUnit));
            SinceAtStay.Add(Since(stayingUnit));
            if (WhenStays != null)
            {
                WhenStays(stayingUnit);
            }
        }

        /// <summary>Logs the unit.</summary>
        public override void OnUnitLeft(Unit leavingUnit)
        {
            OwnerSeen = Owner.Current;
            Log.Add("left " + Label(leavingUnit));
            if (WhenLeft != null)
            {
                WhenLeft(leavingUnit);
            }
        }
    }

    /// <summary>An area that never sets its Shape.</summary>
    public sealed class ShapelessArea : Area
    {
    }
}
