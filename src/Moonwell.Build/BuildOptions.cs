namespace Moonwell.Build
{
    /// <summary>What a build is asked to do.</summary>
    public sealed class BuildOptions
    {
        /// <summary>True for a release build (RELEASE in place of DEBUG, comments removed); false for a development build.</summary>
        public bool Release { get; init; }
    }
}
