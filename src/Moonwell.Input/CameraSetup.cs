namespace Moonwell
{
    /// <summary>A camera placed in World Editor, to apply to a player's camera with <c>Camera.Apply</c>. Get one by its name with <see cref="Named"/>; the build checks the names against the map's cameras.</summary>
    public sealed class CameraSetup
    {
        /// <summary>Wraps a setup the game made.</summary>
        internal CameraSetup(string name, camerasetup handle)
        {
            Name = name;
            Handle = handle;
        }

        /// <summary>The camera's name in World Editor.</summary>
        [LocalSafe]
        public string Name { get; }

        /// <summary>The game's setup.</summary>
        internal camerasetup Handle { get; }

        /// <summary>The camera World Editor has under this name; the same object each time. Throws ArgumentException when the map has no camera of that name, and InvalidOperationException before the package starts.</summary>
        [LocalSafe]
        public static CameraSetup Named(string name)
        {
            return InputSystem.SetupNamed(name);
        }
    }
}
