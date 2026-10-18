using Moonwell;

namespace Sample
{
    /// <summary>Prints one line when the game starts, naming the kind of build.</summary>
    public sealed class Hello : GameComponent
    {
        public override void OnGameStart()
        {
#if DEBUG
            Natives.DisplayTextToPlayer(Natives.Player(0), 0f, 0f, "sample: started (development)");
#else
            Natives.DisplayTextToPlayer(Natives.Player(0), 0f, 0f, "sample: started (release)");
#endif
        }
    }
}
