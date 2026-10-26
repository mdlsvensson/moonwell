namespace Moonwell.Engine
{
    /// <summary>The scan, once a second while items are tracked, for items the game made with no Moonwell code, such as a creep's drop: a take-in hook cannot hear of those.</summary>
    internal sealed class ItemScan : IDue
    {
        /// <summary>The step of the next scan; -1 while none is on the cycle's ring.</summary>
        private int dueStep = -1;

        /// <summary>The scan runs in the second pass, with the cell checks.</summary>
        public int Pass
        {
            get { return 1; }
        }

        /// <summary>The step of the next scan; -1 when there is none.</summary>
        public int DueStep
        {
            get { return dueStep; }
        }

        /// <summary>Puts the scan on the ring for a second from now, unless it is on it.</summary>
        public void Arm()
        {
            if (dueStep >= 0)
            {
                return;
            }

            var beat = Tracking.Beat;
            dueStep = beat.StepNow + Cycle.Steps(1.0f);
            beat.Schedule(this, dueStep);
        }

        /// <summary>Takes in and tracks the items of the world that are not tracked yet, and puts the scan on the ring for a second on. An error it raises is reported as Interactions.ItemScan: nothing leaves the cycle's step.</summary>
        public void RunDue(int step)
        {
            dueStep = -1;
            var failure = Guard.Scan(this);
            Arm();
            if (failure != null)
            {
                Errors.Report("Interactions.ItemScan", failure);
            }
        }

        /// <summary>The scan itself, which RunDue runs guarded.</summary>
        internal void ScanWorld()
        {
            Tracking.TrackItemsInWorld();
        }
    }
}
