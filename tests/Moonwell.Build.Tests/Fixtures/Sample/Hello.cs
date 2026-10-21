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

        /// <summary>
        /// Makes a footman facing the default way and tells the player; nothing calls it. The build's tests read its Lua: the
        /// defaults it leaves out and Moonwell's const come from Moonwell's library, so they show the build resolves it (C18).
        /// </summary>
        public static Unit Footman(Player player)
        {
            player.Print("sample: a footman, facing " + Unit.DefaultFacing);
            return Unit.Create(ObjectTypes.FourCC("hfoo"), player, 0f, 0f);
        }
    }
}
