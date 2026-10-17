using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>The runtime's static state: the world it talks to, and the state each task adds.</summary>
    public static class Runtime
    {
        /// <summary>The world the runtime talks to; set by Reset.</summary>
        internal static IWorld World = null!;

        /// <summary>The owner of everything no other owner holds; set by Reset.</summary>
        internal static Owner Root = null!;

        /// <summary>The scheduler that runs the timers; set by Reset.</summary>
        internal static Scheduler Scheduler = null!;

        /// <summary>Makes a world the runtime's world and clears the state of every task.</summary>
        internal static void Reset(IWorld world)
        {
            World = world;
            Errors.Reset();
            Hooks.Reset();
            Events.Reset();
            Scheduler = new Scheduler();
            Root = new RootOwner();
            Owner.current = null;
        }

        /// <summary>Called when a wake request is due; runs the timers that are due.</summary>
        internal static void Wake()
        {
            Scheduler.Wake();
        }
    }
}
