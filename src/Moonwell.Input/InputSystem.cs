using System;
using Moonwell.Engine;
using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>Starts the input package; a map's generated Main calls Install when the map uses the package, with whether the map names the mouse and the World Editor cameras the build wrote.</summary>
    public static class InputSystem
    {
        /// <summary>The world the package talks to; null until Install.</summary>
        private static IInputWorld? world;

        /// <summary>Each slot's mouse; null until Install.</summary>
        private static Mouse[]? mice;

        /// <summary>The x of the world's top-right corner less 32, a reading the game gives at times past the map's edge.</summary>
        private static float cornerX;

        /// <summary>The y of the world's top-right corner less 32.</summary>
        private static float cornerY;

        /// <summary>
        /// Installs the package on the game. With the mouse, it listens for the mouse of every user player playing now.
        /// The camera names and their numbers are World Editor's cameras, as the build wrote them.
        /// </summary>
        public static void Install(bool mouse, string[] cameraNames, float[] cameras)
        {
            Install(new NativeInputWorld(), mouse, cameraNames, cameras);
        }

        /// <summary>Installs the package on a world; tests give a fake one. With the mouse, it listens for the mouse of every user player playing now, in slot order.</summary>
        public static void Install(IInputWorld host, bool mouse, string[] cameraNames, float[] cameras)
        {
            world = host;
            mice = new Mouse[Player.MaxSlots];
            for (var slot = 0; slot < Player.MaxSlots; slot++)
            {
                mice[slot] = new Mouse(slot);
            }

            MouseReactions.Reset();
            if (!mouse)
            {
                return;
            }

            host.WorldTopRight(out var maxX, out var maxY);
            cornerX = maxX - 32f;
            cornerY = maxY - 32f;
            var users = Player.Users;
            for (var i = 0; i < users.Count; i++)
            {
                host.ListenMouse(users[i].Handle, MouseReactions.OnMoved, MouseReactions.OnPressed, MouseReactions.OnReleased);
            }
        }

        /// <summary>The world the package talks to.</summary>
        internal static IInputWorld World
        {
            get
            {
                if (world == null)
                {
                    throw NotInstalled();
                }

                return world;
            }
        }

        /// <summary>The x of the reading that is the world's top-right corner less 32.</summary>
        internal static float CornerX => cornerX;

        /// <summary>The y of the reading that is the world's top-right corner less 32.</summary>
        internal static float CornerY => cornerY;

        /// <summary>The mouse of a slot. Throws before Install.</summary>
        internal static Mouse MouseOf(int slot)
        {
            if (mice == null)
            {
                throw NotInstalled();
            }

            return mice[slot];
        }

        /// <summary>Forgets the install, so a case can check what reads before it do.</summary>
        internal static void Uninstall()
        {
            world = null;
            mice = null;
            MouseReactions.Reset();
        }

        /// <summary>The error of a read before Install.</summary>
        private static InvalidOperationException NotInstalled()
        {
            return new InvalidOperationException("Moonwell.Input is not installed: the build installs it in a map that references the package");
        }
    }
}
