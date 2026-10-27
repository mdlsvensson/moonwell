#if DEBUG
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>The packages' views of the debug mode, in the order they were added, and the length of a step.</summary>
    internal static class DebugViews
    {
        /// <summary>One step of Moonwell's time when no view sets it: 1/32 s.</summary>
        private const float DefaultStep = 0.03125f;

        /// <summary>The views, in the order they were added; cleared by Runtime.Reset.</summary>
        internal static List<IDebugView> All = new List<IDebugView>();

        /// <summary>The seconds of Moonwell's time one step moves: the interactions package's step when it is in the map, else 1/32 s.</summary>
        internal static float StepLength = DefaultStep;

        /// <summary>Adds a view; a view already added is not added again.</summary>
        internal static void Add(IDebugView view)
        {
            if (!All.Contains(view))
            {
                All.Add(view);
            }
        }

        /// <summary>Forgets every view and sets the step back to 1/32 s; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            All.Clear();
            StepLength = DefaultStep;
        }
    }
}
#endif
