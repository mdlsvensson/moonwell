using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A unit the contact cases make: it writes "&lt;Label&gt; struck" and "&lt;Label&gt; damaged" to the test log, records the damage it took and the owner current at OnStruck, and runs the actions a case gives it. It starts with a QuietTarget, which writes no OnStruck.</summary>
    public class Struck : Unit, IMissileTarget
    {
        /// <summary>The name it logs with.</summary>
        public string Label = "unit";

        /// <summary>The tag of each damage it took, in the order taken.</summary>
        public List<object?> Tags = new List<object?>();

        /// <summary>The source of each damage it took, in the order taken.</summary>
        public List<Unit?> Sources = new List<Unit?>();

        /// <summary>The owner current when OnStruck last ran.</summary>
        public Scope? StruckWith;

        /// <summary>Run at the end of OnStruck; null for nothing.</summary>
        public Action<Impact>? Striking;

        /// <summary>Run at the end of OnDamaged; null for nothing.</summary>
        public Action<Damage>? Damaging;

        /// <summary>Makes a unit with its component that writes no OnStruck.</summary>
        public Struck()
        {
            Components.Add(new QuietTarget());
        }

        /// <summary>Logs the strike and keeps the current owner, then runs the action.</summary>
        public void OnStruck(Impact impact)
        {
            TestLog.Lines.Add(Label + " struck");
            StruckWith = Scope.Current;
            if (Striking != null)
            {
                Striking(impact);
            }
        }

        /// <summary>Logs the damage and keeps its tag and source, then runs the action.</summary>
        public override void OnDamaged(Damage damage)
        {
            TestLog.Lines.Add(Label + " damaged");
            Tags.Add(damage.Tag);
            Sources.Add(damage.Source);
            if (Damaging != null)
            {
                Damaging(damage);
            }
        }
    }
}
