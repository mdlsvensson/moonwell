#if DEBUG
namespace Moonwell.Engine
{
    /// <summary>What the debug mode counts of one class of interaction: how many have started and not ended, their pairs, and their ticks in each of the last 32 steps. The cycle counts into plain int fields; the view reads them.</summary>
    internal sealed class InteractionStats
    {
        /// <summary>The class, as Names.ClassOf gives it, so two classes of one name stay apart.</summary>
        public object Class;

        /// <summary>The class's name.</summary>
        public string Name;

        /// <summary>How many interactions of the class have started and not ended.</summary>
        public int Count;

        /// <summary>How many pairs they have.</summary>
        public int Pairs;

        /// <summary>The OnTick runs of the step running.</summary>
        public int StepTicks;

        /// <summary>The OnTick runs of each of the last 32 steps, at the view's places for the steps.</summary>
        public int[] Ticks;

        /// <summary>Makes the counts of a class, none yet, with a place for each of the steps kept.</summary>
        public InteractionStats(object kind, string name, int steps)
        {
            Class = kind;
            Name = name;
            Ticks = new int[steps];
        }
    }
}
#endif
