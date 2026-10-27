using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A map's area with fields of its own named as Area's: CSharp.lua keeps them apart from the base's only when it sees the base's, which it does not for a private one.</summary>
    public sealed class ShadowArea : Area
    {
        private readonly string shape = "own shape";

        private readonly int tick = 7;

        private readonly List<string> inside = new List<string>();

        private readonly List<string> since = new List<string>();

        private readonly List<string> away = new List<string>();

        private bool active;

        private readonly List<string> refused = new List<string>();

        /// <summary>The reactions so far, in order.</summary>
        public List<string> Log = new List<string>();

        /// <summary>Makes an area of a shape that stays every second.</summary>
        public ShadowArea(AreaShape area)
        {
            Shape = area;
            Tick = 1f;
        }

        /// <summary>The area's own fields, as one line.</summary>
        public string Own()
        {
            return shape + ", " + tick + ", " + inside.Count + ", " + since.Count + ", " + away.Count + ", " + (active ? "active" : "idle") + ", " + refused.Count;
        }

        /// <summary>Logs the unit and changes its own fields.</summary>
        public override void OnUnitEntered(Unit enteringUnit)
        {
            Log.Add("entered");
            inside.Add("a");
            since.Add("b");
            away.Add("c");
            refused.Add("d");
            active = true;
        }

        /// <summary>Logs the stay.</summary>
        public override void OnUnitStays(Unit stayingUnit)
        {
            Log.Add("stays");
        }

        /// <summary>Logs the unit and changes its own fields.</summary>
        public override void OnUnitLeft(Unit leavingUnit)
        {
            Log.Add("left");
            active = false;
        }
    }
}
