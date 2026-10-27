namespace Moonwell
{
    /// <summary>A camera placed in World Editor, to apply to a player's camera with <c>Camera.Apply</c>. Get one by its name with <see cref="Named"/>; the build checks the names against the map's cameras.</summary>
    public sealed class CameraSetup
    {
        /// <summary>The game's setup; null for one named before the package started, which is looked up by its name when used.</summary>
        private readonly camerasetup? handle;

        /// <summary>Wraps a setup the game made.</summary>
        internal CameraSetup(string name, camerasetup handle)
        {
            Name = name;
            this.handle = handle;
        }

        /// <summary>Makes a setup named before the package started, looked up by its name when used.</summary>
        private CameraSetup(string name)
        {
            Name = name;
        }

        /// <summary>The camera's name in World Editor.</summary>
        [LocalSafe]
        public string Name { get; }

        /// <summary>The game's setup: for one named before the package started, the installed camera of its name, looked up at each use. Throws ArgumentException when the map has no camera of that name, and InvalidOperationException before the package starts.</summary>
        internal camerasetup Handle
        {
            get { return handle ?? InputSystem.SetupNamed(Name).Handle; }
        }

        /// <summary>
        /// The camera World Editor has under this name. Once the package has started it is the same object each time, and a
        /// name the map has no camera of throws ArgumentException. Named before the package starts (in a field of a component
        /// listed in the map's <c>Components</c>, which is made before the packages start), it gives a setup that is looked up
        /// when applied, and an unknown name throws ArgumentException there.
        /// </summary>
        [LocalSafe]
        public static CameraSetup Named(string name)
        {
            if (!InputSystem.Installed)
            {
                return new CameraSetup(name);
            }

            return InputSystem.SetupNamed(name);
        }
    }
}
