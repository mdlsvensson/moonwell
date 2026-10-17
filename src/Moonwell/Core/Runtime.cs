using System;
using Moonwell.Engine;
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
            Scheduler = new Scheduler();
            Hooks.Reset();
            Events.Reset();
            Game.Instance = new Game();
            Root = Game.Instance;
            Player.Reset();
            Units.Reset();
            Items.Reset();
            Destructables.Reset();
            ObjectTypes.Reset();
            Owner.current = null;
            Timer.Every(0.25f, Sweep.Run, Game.Instance);
        }

        /// <summary>Called when a wake request is due; runs the timers that are due.</summary>
        internal static void Wake()
        {
            Scheduler.Wake();
        }

        /// <summary>Starts the runtime on the game; the map's Main calls it once, with the setup that registers its types and components.</summary>
        public static void Start(Action setup)
        {
            Reset(new NativeWorld());
            NativeWorld.Later(() => Begin(setup));
        }

        /// <summary>Runs the setup, then takes in what is already in the world and starts the game's components.</summary>
        private static void Begin(Action setup)
        {
            var previous = Owner.current;
            Owner.current = Game.Instance;
            try
            {
                setup();
            }
            catch (Exception error)
            {
                Errors.Report("Moonwell.Setup", error);
            }

            Owner.current = previous;

            foreach (var handle in World.UnitsInWorld())
            {
                try
                {
                    Units.TakeIn(handle, null);
                }
                catch (Exception error)
                {
                    Errors.Report("Moonwell.Start", error);
                }
            }

            foreach (var handle in World.ItemsInWorld())
            {
                if (ObjectTypes.ItemMaker(World.ItemTypeId(handle)) != null)
                {
                    try
                    {
                        Items.TakeIn(handle, null);
                    }
                    catch (Exception error)
                    {
                        Errors.Report("Moonwell.Start", error);
                    }
                }
            }

            foreach (var handle in World.DestructablesInWorld())
            {
                if (ObjectTypes.DestructableMaker(World.DestructableTypeId(handle)) != null)
                {
                    try
                    {
                        Destructables.TakeIn(handle, null);
                    }
                    catch (Exception error)
                    {
                        Errors.Report("Moonwell.Start", error);
                    }
                }
            }

            try
            {
                Inbound.GameStarted();
            }
            catch (Exception error)
            {
                Errors.Report("Moonwell.Start", error);
            }
        }
    }
}
