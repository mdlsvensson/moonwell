namespace Moonwell.Engine
{
    /// <summary>A gizmo's place in the cycle: the step it ticks at next. A gizmo is on the ring once at a time.</summary>
    internal sealed class GizmoDue : IDue
    {
        /// <summary>The gizmo that ticks.</summary>
        private readonly Gizmo gizmo;

        /// <summary>The step of the gizmo's next tick; -1 while it is not on the ring or has ended.</summary>
        private int dueStep = -1;

        /// <summary>Whether the gizmo has ended; it then ticks no more, and its entry on the ring is dropped when the ring comes to it.</summary>
        private bool cancelled;

        /// <summary>Makes the due of a gizmo, not on the ring yet.</summary>
        public GizmoDue(Gizmo gizmo)
        {
            this.gizmo = gizmo;
        }

        /// <summary>Gizmos run in the first pass.</summary>
        public int Pass
        {
            get { return 0; }
        }

        /// <summary>The step of the next tick; -1 when there is none.</summary>
        public int DueStep
        {
            get { return dueStep; }
        }

        /// <summary>Puts the gizmo on the ring for its next tick, a tick from now, unless it is on the ring already, ticks no more or has ended. A tick set to 0 and back before its entry came up stays where it is.</summary>
        public void Arm()
        {
            if (dueStep >= 0 || cancelled || gizmo.Tick <= 0.0f)
            {
                return;
            }

            var beat = Tracking.Beat;
            dueStep = beat.StepNow + Cycle.Steps(gizmo.Tick);
            beat.Schedule(this, dueStep);
        }

        /// <summary>Ends the gizmo's ticking for good.</summary>
        public void Cancel()
        {
            cancelled = true;
            dueStep = -1;
        }

        /// <summary>Ticks the gizmo, and puts it on the ring again for a tick from now, with the tick it has after, unless it has ended or no longer ticks.</summary>
        public void RunDue(int step)
        {
            dueStep = -1;
            if (cancelled || gizmo.Tick <= 0.0f)
            {
                return;
            }

            gizmo.CallTick();
            Arm();
        }
    }
}
