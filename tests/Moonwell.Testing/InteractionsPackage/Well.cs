using System;
using System.Collections.Generic;
using Moonwell.Engine;

namespace Moonwell.Testing
{
    /// <summary>A gizmo the interactions cases make: it counts its OnCreated and OnLeave, notes the cycle's step at each OnTick, and runs the actions a case gives it in its reactions.</summary>
    public class Well : Gizmo
    {
        /// <summary>How many times OnCreated ran.</summary>
        public int Created;

        /// <summary>How many times OnLeave ran.</summary>
        public int Left;

        /// <summary>The cycle's step at each OnTick, in turn.</summary>
        public List<int> TickSteps = new List<int>();

        /// <summary>Whether OnTick throws an InvalidOperationException after it has noted its step and run its action.</summary>
        public bool Throws;

        /// <summary>An action OnCreated runs after it has counted; null for none.</summary>
        public Action? Creating;

        /// <summary>An action OnLeave runs after it has counted; null for none.</summary>
        public Action? Leaving;

        /// <summary>An action OnTick runs after it has noted its step; null for none.</summary>
        public Action? Ticking;

        /// <summary>Makes a well that ticks every tick seconds; 0 for none.</summary>
        public Well(float tick = 0.0f)
        {
            Tick = tick;
        }

        /// <summary>Sets the well's tick, which only the gizmo itself may.</summary>
        public void SetTick(float seconds)
        {
            Tick = seconds;
        }

        /// <summary>Counts the call, then runs the action.</summary>
        protected override void OnCreated()
        {
            Created++;
            if (Creating != null)
            {
                Creating();
            }
        }

        /// <summary>Counts the call, then runs the action.</summary>
        protected override void OnLeave()
        {
            Left++;
            if (Leaving != null)
            {
                Leaving();
            }
        }

        /// <summary>Notes the cycle's step, then runs the action, then throws if it was told to.</summary>
        public override void OnTick()
        {
            TickSteps.Add(Tracking.StepNow);
            if (Ticking != null)
            {
                Ticking();
            }

            if (Throws)
            {
                throw new InvalidOperationException("boom");
            }
        }
    }
}
