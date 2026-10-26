using System;
using System.Collections.Generic;
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

        /// <summary>Each slot's camera; null until Install.</summary>
        private static Camera[]? eachCamera;

        /// <summary>World Editor's cameras as setups, in the order the build wrote them; null until Install.</summary>
        private static List<CameraSetup>? setups;

        /// <summary>The x of the world's top-right corner less 32, a reading the game gives at times past the map's edge.</summary>
        private static float cornerX;

        /// <summary>The y of the world's top-right corner less 32.</summary>
        private static float cornerY;

        /// <summary>
        /// Installs the package on the game. With the mouse, it listens for the mouse of every user player playing now.
        /// The camera names and their numbers are World Editor's cameras, as the build wrote them: for each camera in the
        /// names' order, the target x and y, the count n, n pairs of field code and value, and the camera type (-1 for
        /// none). Throws ArgumentException when the numbers do not hold exactly the cameras the names say.
        /// </summary>
        public static void Install(bool mouse, string[] cameraNames, float[] cameras)
        {
            Install(new NativeInputWorld(), mouse, cameraNames, cameras);
        }

        /// <summary>Installs the package on a world; tests give a fake one. With the mouse, it listens for the mouse of every user player playing now, in slot order.</summary>
        public static void Install(IInputWorld host, bool mouse, string[] cameraNames, float[] cameras)
        {
            var starts = CameraStarts(cameraNames, cameras);
            world = host;
            mice = new Mouse[Player.MaxSlots];
            eachCamera = new Camera[Player.MaxSlots];
            for (var slot = 0; slot < Player.MaxSlots; slot++)
            {
                mice[slot] = new Mouse(slot);
                eachCamera[slot] = new Camera(slot);
            }

            setups = new List<CameraSetup>();
            for (var i = 0; i < cameraNames.Length; i++)
            {
                setups.Add(new CameraSetup(cameraNames[i], host.MakeSetup(cameras, starts[i])));
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

        /// <summary>The camera of a slot. Throws before Install.</summary>
        internal static Camera CameraOf(int slot)
        {
            if (eachCamera == null)
            {
                throw NotInstalled();
            }

            return eachCamera[slot];
        }

        /// <summary>The setup World Editor's camera of this name made, or ArgumentException when none has it. Throws before Install.</summary>
        internal static CameraSetup SetupNamed(string name)
        {
            if (setups == null)
            {
                throw NotInstalled();
            }

            for (var i = 0; i < setups.Count; i++)
            {
                if (setups[i].Name == name)
                {
                    return setups[i];
                }
            }

            throw new ArgumentException("No World Editor camera is named '" + name + "'.");
        }

        /// <summary>Where each camera starts in the numbers, walking them as the build wrote them. Throws ArgumentException unless the numbers end exactly where the last camera's do.</summary>
        private static int[] CameraStarts(string[] cameraNames, float[] cameras)
        {
            var starts = new int[cameraNames.Length];
            var at = 0;
            for (var i = 0; i < cameraNames.Length; i++)
            {
                starts[i] = at;
                if (at + 3 > cameras.Length)
                {
                    throw WrongCameraData(cameraNames.Length);
                }

                var pairs = (int)cameras[at + 2];
                if (pairs < 0)
                {
                    throw WrongCameraData(cameraNames.Length);
                }

                at = at + 3 + pairs * 2 + 1;
                if (at > cameras.Length)
                {
                    throw WrongCameraData(cameraNames.Length);
                }
            }

            if (at != cameras.Length)
            {
                throw WrongCameraData(cameraNames.Length);
            }

            return starts;
        }

        /// <summary>The error of camera numbers that do not hold the cameras the names say.</summary>
        private static ArgumentException WrongCameraData(int count)
        {
            return new ArgumentException("The camera numbers do not hold exactly the " + count + " camera(s) the names say.");
        }

        /// <summary>Forgets the install, so a case can check what reads before it do.</summary>
        internal static void Uninstall()
        {
            world = null;
            mice = null;
            eachCamera = null;
            setups = null;
            MouseReactions.Reset();
        }

        /// <summary>The error of a read before Install.</summary>
        private static InvalidOperationException NotInstalled()
        {
            return new InvalidOperationException("Moonwell.Input is not installed: the build installs it in a map that references the package");
        }
    }
}
