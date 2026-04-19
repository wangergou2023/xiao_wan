var client = new HttpClient();

var urlParams = new URLSearchParams(window.location.search);
esn = urlParams.get("serial");

showFaceButtons = false;

var areThereFaces = false;

function refreshFaceList() {
  var x = document.getElementById("faceList");
  x.innerHTML = "";
  fetch("/api-sdk/get_faces?serial=" + esn)
    .then((response) => response.text())
    .then((response) => {
      if (response.includes("null")) {
        showFaceButtons = false;
        var option = document.createElement("option");
        option.text = "还没有已记录的人脸";
        option.value = "none";
        areThereFaces = false;
        x.add(option);
        document.getElementById("faceButtons").style.display = "none";
      } else {
        areThereFaces = true;
        jsonResp = JSON.parse(response);
        showFaceButtons = true;
        for (var i = 0; i < jsonResp.length; i++) {
          var option = document.createElement("option");
          option.text = jsonResp[i]["name"];
          option.value = jsonResp[i]["face_id"] + ":" + jsonResp[i]["name"];
          x.add(option);
        }
        if (showFaceButtons == true) {
          document.getElementById("faceButtons").style.display = "block";
        } else {
          document.getElementById("faceButtons").style.display = "none";
        }
      }
    });
}

refreshFaceList();

/*
function showFaceSection() {
  id = "section-faces"
  var headings = document.getElementsByClassName("toggleable-section");
  for (var i = 0; i < headings.length; i++) {
      headings[i].style.display = "none";
  }
  document.getElementById(id).style.display = "block";
  console.log(showFaceButtons)
  if (showFaceButtons == true) {
    document.getElementById("faceButtons").style.display = "block";
  } else {
    document.getElementById("faceButtons").style.display = "none";
  }
}
*/

function renameFace() {
  if (!areThereFaces) {
    window.alert("请先录入一个人脸。");
  } else {
    var x = document.getElementById("faceList");
    oldFaceName = x.value.split(":")[1];
    faceId = x.value.split(":")[0];
    newFaceName = window.prompt("请输入新名字：");
    if (newFaceName == "") {
      window.alert("名字不能为空。");
    } else {
      fetch(
        "/api-sdk/rename_face?serial=" +
          esn +
          "&oldname=" +
          oldFaceName +
          "&id=" +
          faceId +
          "&newname=" +
          newFaceName
      ).then(function () {
        alert("修改成功。");
        refreshFaceList();
      });
    }
  }
}

function addFace() {
  var name = document.getElementById("faceInput").value;
  if (name == "") {
    alert("请输入名字。");
    return;
  } else {
    fetch("/api-sdk/add_face?serial=" + esn + "&name=" + name).then(
      function () {
        alert(
          "请求已发送。现在让机器人面对人脸完成录入。"
        );
        refreshFaceList();
      }
    );
  }
}

function deleteFace() {
  if (!areThereFaces) {
    window.alert("请先录入一个人脸。");
  } else {
    var x = document.getElementById("faceList");
    faceId = x.value.split(":")[0];
    fetch("/api-sdk/delete_face?serial=" + esn + "&id=" + faceId).then(
      function () {
        alert("删除成功。");
        refreshFaceList();
      }
    );
  }
}
