using System.Collections.Generic;
using Moonwell;

namespace Desyncs
{
    /// <summary>One multiplayer mistake per analyzer rule, each on a line the build test names.</summary>
    public sealed class Mistakes : GameComponent
    {
        public override void OnGameStart()
        {
            var player = Player.Get(0);
            Local.For(player, () => Natives.CreateUnit(player.Handle, 0, 0f, 0f, 0f));
            var x = Natives.GetCameraTargetPositionX();
            var id = Natives.GetHandleId(player.Handle);
            foreach (var pair in new Dictionary<int, int>())
            {
            }

            long total = Natives.GetPlayerId(player.Handle);
        }
    }
}
