// Copyright (c) 2019 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package sftp

// sftp put/pull function
// NOTE: リモートマシンからリモートマシンにコピーさせるような処理や、対象となるホストを個別に指定してコピーできるような仕組みをつくること！
// TODO(blacknon): 転送時の進捗状況を表示するプログレスバーの表示はさせること
func (r *RunSftp) copy(args []string) {
	// finished := make(chan bool)

	// // set target list
	// targetList := []string{}
	// switch mode {
	// case "push":
	//  targetList = r.To.Server
	// case "pull":
	//  targetList = r.From.Server
	// }

	// for _, value := range targetList {
	//  target := value
	// }
}
