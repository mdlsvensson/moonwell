namespace Moonwell.Engine
{
    /// <summary>Work the cycle runs at a step: a gizmo's tick, a cell check or an interaction's check. The cycle keeps it in a list of its step until that step comes.</summary>
    internal interface IDue
    {
        /// <summary>Which of a step's three passes runs it: 0 for gizmos, 1 for cell checks, 2 for interactions.</summary>
        int Pass { get; }

        /// <summary>The step it runs at; below 0 when it has nothing more to run, and the cycle then drops it from its list.</summary>
        int DueStep { get; }

        /// <summary>Runs the work of a step. The cycle has taken it off its list, so it schedules itself again for a later step if it has more to do.</summary>
        void RunDue(int step);
    }
}
