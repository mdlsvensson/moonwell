namespace Moonwell.Engine
{
    /// <summary>An interaction's place in the cycle: the step of its next check. An interaction is on the ring once at a time; a paused one drops off at its next step and Resume puts it on again.</summary>
    internal sealed class InteractionDue : IDue
    {
        /// <summary>The interaction that checks.</summary>
        private readonly Interaction interaction;

        /// <summary>The step of the next check; -1 while it is not on the ring or has ended.</summary>
        private int dueStep = -1;

        /// <summary>Whether the interaction has ended; it then checks no more, and its entry on the ring is dropped when the ring comes to it.</summary>
        private bool cancelled;

        /// <summary>Makes the due of an interaction, not on the ring yet.</summary>
        public InteractionDue(Interaction interaction)
        {
            this.interaction = interaction;
        }

        /// <summary>Interactions run in the third pass, after the gizmos and the cell checks.</summary>
        public int Pass
        {
            get { return 2; }
        }

        /// <summary>The step of the next check; -1 when there is none.</summary>
        public int DueStep
        {
            get { return dueStep; }
        }

        /// <summary>Puts the interaction on the ring for its next check, a tick from now, unless it is on the ring already or has ended.</summary>
        public void Arm()
        {
            if (dueStep >= 0 || cancelled)
            {
                return;
            }

            var beat = Tracking.Beat;
            dueStep = beat.StepNow + Cycle.Steps(interaction.Tick);
            beat.Schedule(this, dueStep);
        }

        /// <summary>Ends the interaction's checks for good.</summary>
        public void Cancel()
        {
            cancelled = true;
            dueStep = -1;
        }

        /// <summary>Runs the check, and puts the interaction on the ring again for a tick from now, unless it has ended. A paused one is left off the ring until Resume.</summary>
        public void RunDue(int step)
        {
            dueStep = -1;
            if (cancelled || interaction.IsPaused)
            {
                return;
            }

            interaction.RunCheck();
            Arm();
        }
    }
}
