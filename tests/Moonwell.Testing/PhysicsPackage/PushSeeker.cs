using System;

namespace Moonwell.Testing
{
    /// <summary>A gizmo that ticks every step and runs the action a case gives it, so a case can look at the grid in the gizmos' pass of a step, after the physics pass of the same step.</summary>
    public sealed class PushSeeker : Gizmo
    {
        /// <summary>What runs at each tick; null for nothing.</summary>
        public Action? Ticked;

        /// <summary>Makes a gizmo that ticks every step.</summary>
        public PushSeeker()
        {
            Tick = Interactions.Step;
        }

        /// <summary>Runs Ticked.</summary>
        public override void OnTick()
        {
            if (Ticked != null)
            {
                Ticked();
            }
        }
    }
}
