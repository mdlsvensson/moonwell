using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>Cases for the full audit's fixes to input: which mouse events the package listens to, the watchers it runs, and World Editor's cameras named before it starts.</summary>
    public static class AuditInputCases
    {
        /// <summary>Makes slots 0 and 1 user players.</summary>
        private static void TwoUsers(FakeWorld world)
        {
            world.SetSlot(0, SlotState.Playing, Controller.User);
            world.SetSlot(1, SlotState.Playing, Controller.User);
        }

        /// <summary>With the mouse on and its moves off (a map whose code reads no synced position and writes no OnMouseMoved), the package listens to each user's presses and releases and not to their moves; a press still gives the position and the button.</summary>
        public static void NoMoves(FakeWorld world)
        {
            TwoUsers(world);
            var fake = new FakeInputWorld();
            InputSystem.Install(fake, true, false, new string[0], new float[0]);
            Expect.Equal(2, fake.Listens.Count, "both users are listened to");
            for (var i = 0; i < fake.Listens.Count; i++)
            {
                Expect.True(fake.Listens[i].Moved == null, "no move event for user " + i);
                Expect.True(fake.Listens[i].Pressed != null && fake.Listens[i].Released != null, "presses and releases for user " + i);
            }

            var mouse = Player.One.Mouse();
            fake.Press(Player.One, MouseButton.Left, 300f, 400f);
            Expect.True(mouse.IsDown(MouseButton.Left), "the press is heard");
            Expect.Near(300f, mouse.X, "and gives the position");
            fake.Release(Player.One, MouseButton.Left, 310f, 410f);
            Expect.True(!mouse.IsDown(MouseButton.Left), "the release is heard");
        }

        /// <summary>With the mouse and its moves on, all three events are listened to; with the mouse off, none, whatever the moves say.</summary>
        public static void Moves(FakeWorld world)
        {
            TwoUsers(world);
            var fake = new FakeInputWorld();
            InputSystem.Install(fake, true, true, new string[0], new float[0]);
            Expect.True(fake.Listens.Count == 2 && fake.Listens[0].Moved != null && fake.Listens[1].Moved != null, "moves for both users");
            fake.Move(Player.Two, 50f, 60f);
            Expect.Near(50f, Player.Two.Mouse().X, "a move is heard");
            var off = new FakeInputWorld();
            InputSystem.Install(off, false, true, new string[0], new float[0]);
            Expect.Equal(0, off.Listens.Count, "no mouse, no events");
        }
    }
}
