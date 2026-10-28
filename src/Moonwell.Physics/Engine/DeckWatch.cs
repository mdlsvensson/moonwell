namespace Moonwell.Engine
{
    /// <summary>The package's component on a placed bridge: when the bridge dies or leaves the game, its deck stops being ground. Core watches the bridge's death because this class writes OnDeath.</summary>
    internal sealed class DeckWatch : DestructableComponent
    {
        /// <summary>The deck it watches over.</summary>
        private readonly Deck deck;

        /// <summary>Makes the watch of a deck.</summary>
        internal DeckWatch(Deck deck)
        {
            this.deck = deck;
        }

        /// <summary>The bridge died: its deck is gone.</summary>
        public override void OnDeath()
        {
            Decks.Gone(deck);
        }

        /// <summary>The bridge left the game, or the watch was removed: its deck is gone.</summary>
        protected override void OnRemoved()
        {
            Decks.Gone(deck);
        }
    }
}
