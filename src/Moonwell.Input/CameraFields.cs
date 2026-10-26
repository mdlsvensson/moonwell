namespace Moonwell
{
    /// <summary>Moonwell's own numbers for the camera's fields, which the world maps to the game's constants and the build writes into World Editor's cameras.</summary>
    internal static class CameraFields
    {
        /// <summary>The distance to the target.</summary>
        internal const int Distance = 0;

        /// <summary>The far clipping distance.</summary>
        internal const int FarZ = 1;

        /// <summary>The angle of attack.</summary>
        internal const int AngleOfAttack = 2;

        /// <summary>The field of view.</summary>
        internal const int FieldOfView = 3;

        /// <summary>The roll.</summary>
        internal const int Roll = 4;

        /// <summary>The rotation.</summary>
        internal const int Rotation = 5;

        /// <summary>The height offset.</summary>
        internal const int ZOffset = 6;
    }
}
