using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A missile the contact cases make: it writes "shot impact" and "shot can-hit" to the test log, records what it met and what each impact read inside OnImpact, and runs a question and an action a case gives it.</summary>
    public sealed class Shot : Missile
    {
        /// <summary>The things OnImpact met, in the order it met them; null for the ground.</summary>
        public List<Actor?> Met = new List<Actor?>();

        /// <summary>The impacts, in the order they came.</summary>
        public List<Impact> Impacts = new List<Impact>();

        /// <summary>What each impact read inside OnImpact, eight to an impact: kind, x, y, z, normal x, y, z, speed.</summary>
        public List<float> Reads = new List<float>();

        /// <summary>The damage each impact carried when OnImpact ran.</summary>
        public List<float> Damages = new List<float>();

        /// <summary>The things CanHit was asked about, in the order asked.</summary>
        public List<Actor> Asked = new List<Actor>();

        /// <summary>The answer CanHit gives instead of the default; null for the default.</summary>
        public Func<Actor, bool>? Hit;

        /// <summary>An action OnImpact runs after it has recorded; null for none.</summary>
        public Action<Impact>? Impacting;

        /// <summary>An action OnTick runs; null for none. It ticks only after TickEachStep.</summary>
        public Action? Ticking;

        /// <summary>Makes it tick every step of the cycle, after the physics pass; called before Create.</summary>
        public void TickEachStep()
        {
            Tick = Interactions.Step;
        }

        /// <summary>Runs the action.</summary>
        public override void OnTick()
        {
            if (Ticking != null)
            {
                Ticking();
            }
        }

        /// <summary>Lists a ShotWatch among the components it starts with; called before Create.</summary>
        public void Watch()
        {
            Components.Add(new ShotWatch());
        }

        /// <summary>Records the question, and answers it with Hit, else as the default.</summary>
        public override bool CanHit(Actor other)
        {
            Asked.Add(other);
            TestLog.Lines.Add("shot can-hit");
            if (Hit != null)
            {
                return Hit(other);
            }

            return base.CanHit(other);
        }

        /// <summary>Records the impact and what it read, then runs the action.</summary>
        public override void OnImpact(Impact impact)
        {
            TestLog.Lines.Add("shot impact");
            Met.Add(impact.Other);
            Impacts.Add(impact);
            Reads.Add((float)(int)impact.Kind);
            Reads.Add(impact.X);
            Reads.Add(impact.Y);
            Reads.Add(impact.Z);
            var normal = impact.Normal;
            Reads.Add(normal.X);
            Reads.Add(normal.Y);
            Reads.Add(normal.Z);
            Reads.Add(impact.Speed);
            Damages.Add(impact.Damage);
            if (Impacting != null)
            {
                Impacting(impact);
            }
        }
    }
}
