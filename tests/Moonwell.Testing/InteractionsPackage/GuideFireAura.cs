using System;

namespace Moonwell.Testing
{
    /// <summary>The guide's FireAura as the guide writes it: held by a unit, it burns the units of the holder's enemies within 300 for 10 every half second. It also writes "meet b", "burn b" and "part b" to the test log, with the name Tag gives the other, and a case may turn RecheckAccepts off.</summary>
    public sealed class GuideFireAura : Interaction<Unit, Unit>
    {
        /// <summary>The name the log gives a unit; "?" unless a case says.</summary>
        public Func<Unit, string> Tag = unit => "?";

        /// <summary>Makes the aura, asking Accepts again before each tick unless told not to.</summary>
        public GuideFireAura(bool recheck = true)
        {
            Range = 300.0f;
            Tick = 0.5f;
            RecheckAccepts = recheck;
        }

        /// <summary>The guide's: the holder's enemies.</summary>
        public override bool Accepts(Unit other) => Holder.Owner.IsEnemy(other.Owner);

        /// <summary>Logs "meet".</summary>
        public override void OnMeet(Pair pair)
        {
            TestLog.Lines.Add("meet " + Tag(pair.Other));
        }

        /// <summary>The guide's: 10 damage from the holder. Logs "burn".</summary>
        public override void OnTick(Pair pair)
        {
            Holder.Damage(pair.Other, 10f);
            TestLog.Lines.Add("burn " + Tag(pair.Other));
        }

        /// <summary>Logs "part".</summary>
        public override void OnPart(Pair pair)
        {
            TestLog.Lines.Add("part " + Tag(pair.Other));
        }
    }
}
