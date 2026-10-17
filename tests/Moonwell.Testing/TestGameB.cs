namespace Moonwell.Testing
{
    /// <summary>A game component that counts how many times it was added.</summary>
    public sealed class TestGameB : GameComponent
    {
        /// <summary>How many times OnAdded ran.</summary>
        public int Adds;

        /// <summary>Counts the add.</summary>
        protected override void OnAdded()
        {
            Adds++;
        }
    }
}
