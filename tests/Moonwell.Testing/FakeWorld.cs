namespace Moonwell.Testing
{
    /// <summary>The fake game a case runs against. A placeholder until the fake world is built.</summary>
    public sealed class FakeWorld
    {
        /// <summary>Makes a fresh world for one case.</summary>
        public static FakeWorld Install()
        {
            return new FakeWorld();
        }
    }
}
